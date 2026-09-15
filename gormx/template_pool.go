package gormx

import (
	"cmp"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// templateDatabaseName is the database every Fork clones. It is migrated once,
// then left with zero open connections — PostgreSQL refuses to clone a database
// that any session is still connected to.
const templateDatabaseName = "gormx_template"

// forkMaxIdleConns overrides DefaultDatabaseConfig's 20. That default is tuned
// for a long-lived service holding one database; a fork is one test's private
// database, and 20 idle connections per fork times N concurrent forks is both
// wasted and the fastest way back to "too many clients already".
const forkMaxIdleConns = 2

// templatePoolMaxConnections is prepended to the container's arguments, ahead of
// any the caller passed, so a caller who names max_connections themselves still
// wins (postgres takes the last occurrence). The stock 100 is not a sensible
// default for this type: a pool exists precisely to hand out many databases at
// once, each with its own connection pool.
const templatePoolMaxConnections = 300

// TemplateMigrator prepares the template's schema. It receives both a live
// connection to the template database and that database's DSN — use whichever
// the migration tool needs: an external migrator (Atlas, golang-migrate) takes
// the DSN, AutoMigrate and seed code take the *gorm.DB.
//
// The pool owns the connection and closes it as soon as the migrator returns, so
// the migrator must not retain the *gorm.DB or leave a connection of its own
// open against that DSN. Cloning the template fails outright while any session
// is connected to it, so a leak here surfaces as
// `source database "gormx_template" is being accessed by other users` on the
// first Fork rather than as silent corruption.
//
// The context is the test binary's, not any one test's: the template outlives
// whichever test happened to trigger the migration.
type TemplateMigrator func(ctx context.Context, db *gorm.DB, dsn string) error

// TemplatePool starts one PostgreSQL container, migrates one template database
// inside it, and hands each test a private clone of that template.
//
// It exists because OpenContainer has no reuse: a package with N suites that
// each call it pays N container starts plus N migration runs, which is linear
// and dominated entirely by container startup (~3600ms against ~360ms for a
// full migration in the case that motivated this). Cloning a finished template
// is a file-level copy — tens of milliseconds, and independent of how many
// migrations produced the schema.
//
// Isolation is stronger than the alternative of sharing one database and
// truncating between tests: each test gets its own database, so there is no
// per-package list of tables to keep in sync and no way to forget one. Because
// each test owns its database, tests using a pool are also free to call
// t.Parallel() — Fork deliberately does not call it for them, since whether a
// package's tests may run concurrently depends on the rest of their state
// (t.Setenv panics in a parallel test, package-level fixtures may be shared),
// which only that package can know.
//
// Declare one per test binary and let it start itself:
//
//	var pool = gormx.NewTemplatePool(nil, migrate)
//
//	func TestSomething(t *testing.T) {
//		db := pool.Fork(t)
//		...
//	}
type TemplatePool struct {
	conf    *ContainerConfig
	migrate TemplateMigrator

	// adminDSN, when set, points the pool at a server that is already running
	// instead of a container it starts (see NewTemplatePoolFromDSN).
	adminDSN string
	// template and prefix name this pool's template database and its forks. A
	// container holds exactly one pool, so fixed names serve; a shared server
	// can hold one pool per concurrently running test binary, so external mode
	// makes both unique to the pool.
	template string
	prefix   string

	once        sync.Once
	container   *Container
	admin       *gorm.DB
	adminCloser io.Closer
	baseURL     *url.URL
	startErr    error

	seq atomic.Uint64
}

// NewTemplatePool describes a pool. Nothing runs until the first Fork, so this
// is safe to call from a package-level variable: a package whose tests are all
// filtered out never starts a container. A nil conf uses DefaultContainerConfig.
//
// It panics on a nil migrator rather than deferring the complaint to the first
// Fork — without one the pool is just OpenContainer, and at package-variable
// scope the mistake is worth finding before any test runs.
func NewTemplatePool(conf *ContainerConfig, migrate TemplateMigrator) *TemplatePool {
	if migrate == nil {
		panic("gormx: NewTemplatePool requires a TemplateMigrator")
	}
	return &TemplatePool{conf: conf, migrate: migrate, template: templateDatabaseName, prefix: "fork"}
}

// NewTemplatePoolFromDSN is NewTemplatePool against a PostgreSQL server that is
// already running — a CI service container, a developer's local instance —
// rather than a container the pool starts itself. adminDSN is a postgres:// URL
// for a role that may CREATE DATABASE, naming a database that no template or
// fork will use (typically "postgres").
//
// Test binaries commonly share such a server at the same time (go test -p N), so
// the template and every fork get names unique to this pool, and Close drops the
// template. Forks are still dropped by the test that created them.
func NewTemplatePoolFromDSN(adminDSN string, migrate TemplateMigrator) *TemplatePool {
	if migrate == nil {
		panic("gormx: NewTemplatePoolFromDSN requires a TemplateMigrator")
	}
	if adminDSN == "" {
		panic("gormx: NewTemplatePoolFromDSN requires an admin DSN")
	}
	token := fmt.Sprintf("%d_%08x", os.Getpid(), rand.Uint32())
	return &TemplatePool{
		migrate:  migrate,
		adminDSN: adminDSN,
		template: "gormx_template_" + token,
		prefix:   "fork_" + token,
	}
}

// Fork clones the template into a database of this test's own and returns a
// connection to it. The database and the connection are both torn down when the
// test finishes.
//
// The first call starts the container and migrates the template, so it pays for
// both; every call after that is one CREATE DATABASE. Concurrent first calls are
// serialised, and a startup failure is reported to every test that asks for a
// database, not only the one that happened to be first.
func (p *TemplatePool) Fork(t testing.TB, opts ...gorm.Option) *gorm.DB {
	t.Helper()

	name, dsn := p.fork(t)
	db, closer, err := p.open(t.Context(), dsn, opts...)
	if err != nil {
		t.Fatalf("gormx: failed to connect to fork %s: %+v", name, err)
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("gormx: failed to close fork %s: %+v", name, err)
		}
	})
	return db
}

// ForkDSN clones the template exactly like Fork, but returns the new database's
// DSN instead of a connection. Use it when the code under test must see a
// connection opened the way the application opens its own: Fork's connection
// comes from Open, with this package's defaults and plugins installed (omitted
// associations, soft-delete updated_at, tracing), which an application that
// does not use them should not test against.
//
// The database is dropped when the test finishes. WITH (FORCE) terminates any
// connection the caller left open, so closing them first is tidy but not
// required.
func (p *TemplatePool) ForkDSN(t testing.TB) string {
	t.Helper()

	_, dsn := p.fork(t)
	return dsn
}

// fork creates this test's database from the template, registers its drop, and
// returns its name and DSN.
func (p *TemplatePool) fork(t testing.TB) (name, dsn string) {
	t.Helper()

	p.once.Do(p.start)
	if p.startErr != nil {
		t.Fatalf("gormx: failed to start the template pool: %+v", p.startErr)
	}

	name = p.forkName(t)
	if err := p.admin.WithContext(t.Context()).Exec(
		`CREATE DATABASE ` + name + ` TEMPLATE ` + p.template,
	).Error; err != nil {
		t.Fatalf("gormx: failed to fork the template into %s: %+v", name, err)
	}
	// Registered before any connection's cleanup so it runs after it (t.Cleanup
	// is LIFO). FORCE terminates any session the test left behind — a lifecycle
	// that was never stopped, a background worker — which is fine for a database
	// that exists only for this test.
	t.Cleanup(func() {
		if err := p.admin.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`).Error; err != nil {
			t.Errorf("gormx: failed to drop fork %s: %+v", name, err)
		}
	})
	return name, p.dsn(name)
}

// Close releases the admin connection and terminates the container. Forks clean
// themselves up with their own test, so this only tears down what the pool
// itself holds, and it is a no-op for a pool that never started.
//
// Calling it from TestMain stops the container promptly; skipping it is not a
// leak either, since testcontainers' reaper removes the container when the test
// binary exits.
func (p *TemplatePool) Close(ctx context.Context) error {
	var errs []error
	if p.adminDSN != "" && p.admin != nil {
		// A shared server outlives this test binary; the template must not.
		errs = append(errs, p.admin.WithContext(ctx).Exec(
			`DROP DATABASE IF EXISTS `+p.template+` WITH (FORCE)`,
		).Error)
	}
	if p.adminCloser != nil {
		errs = append(errs, p.adminCloser.Close())
		p.adminCloser = nil
		p.admin = nil
	}
	if p.container != nil {
		errs = append(errs, p.container.Terminate(ctx))
		p.container = nil
	}
	return errors.WithStack(stderrors.Join(errs...))
}

// start runs under p.once. It records its failure rather than returning it, so
// that Fork can report the same error to every test that needs a database.
func (p *TemplatePool) start() {
	ctx := context.Background()
	defer func() {
		if p.startErr != nil {
			if err := p.Close(ctx); err != nil {
				p.startErr = stderrors.Join(p.startErr, err)
			}
		}
	}()

	adminDSN := p.adminDSN
	if adminDSN == "" {
		// Copy rather than mutate: conf belongs to the caller, and
		// DefaultContainerConfig hands out a fresh value they may hold on to.
		conf := *cmp.Or(p.conf, DefaultContainerConfig())
		conf.Args = append(
			[]string{"-c", fmt.Sprintf("max_connections=%d", templatePoolMaxConnections)},
			conf.Args...,
		)

		container, err := OpenContainer(ctx, &conf)
		if err != nil {
			p.startErr = err
			return
		}
		p.container = container
		adminDSN = container.DSN
	}

	var err error
	p.baseURL, err = url.Parse(adminDSN)
	if err != nil {
		p.startErr = errors.Wrap(err, "failed to parse the admin DSN")
		return
	}

	// The admin connection outlives every fork: it is the session that issues
	// CREATE/DROP DATABASE, so it must not be connected to any database being
	// cloned or dropped. The server's default database is that neutral spot.
	p.admin, p.adminCloser, err = p.open(ctx, adminDSN)
	if err != nil {
		p.startErr = errors.Wrap(err, "failed to connect to the admin database")
		return
	}
	if err := p.admin.WithContext(ctx).Exec(`CREATE DATABASE ` + p.template).Error; err != nil {
		p.startErr = errors.Wrap(err, "failed to create the template database")
		return
	}

	templateDSN := p.dsn(p.template)
	template, templateCloser, err := p.open(ctx, templateDSN)
	if err != nil {
		p.startErr = errors.Wrap(err, "failed to connect to the template database")
		return
	}
	migrateErr := p.migrate(ctx, template, templateDSN)
	// Close unconditionally, and before reporting the migration error: a failed
	// migration must not leave the template pinned open either.
	closeErr := templateCloser.Close()
	if migrateErr != nil {
		p.startErr = errors.Wrap(migrateErr, "failed to migrate the template database")
		return
	}
	if closeErr != nil {
		p.startErr = errors.Wrap(closeErr, "failed to release the template database")
	}
}

// open dials one of the container's databases with the package's own defaults,
// so a fork behaves like any other gormx-managed connection (tracing, plugins)
// rather than a bare gorm.Open.
func (p *TemplatePool) open(ctx context.Context, dsn string, opts ...gorm.Option) (*gorm.DB, io.Closer, error) {
	conf := DefaultDatabaseConfig()
	conf.DSN = dsn
	conf.MaxIdleConns = forkMaxIdleConns
	return Open(ctx, conf, opts...)
}

func (p *TemplatePool) dsn(database string) string {
	u := *p.baseURL
	u.Path = "/" + database
	return u.String()
}

var nonIdentifierRunes = regexp.MustCompile(`[^a-z0-9]+`)

// forkName keeps the test's name in the database name so a container left
// running after a failure can be inspected, and prefixes a counter to stay
// unique across subtests that share a name.
func (p *TemplatePool) forkName(t testing.TB) string {
	head := fmt.Sprintf("%s_%d_", p.prefix, p.seq.Add(1))
	name := nonIdentifierRunes.ReplaceAllString(strings.ToLower(t.Name()), "_")
	// PostgreSQL truncates identifiers at 63 bytes. Truncate the test's part
	// here instead, so the pool prefix and counter always survive.
	if room := 63 - len(head); len(name) > room {
		name = name[:room]
	}
	return head + name
}
