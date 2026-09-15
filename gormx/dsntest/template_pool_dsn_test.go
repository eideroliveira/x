// Package dsntest exercises gormx.NewTemplatePoolFromDSN against a PostgreSQL
// server that is already running. It is a package of its own because the gormx
// test binary's TestMain starts a container: these tests exist precisely for
// machines and CI jobs that have a server but no Docker.
package dsntest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/qor5/x/v3/gormx"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Widget struct {
	ID   string `gorm:"primaryKey"`
	Name string
}

const seedWidget = "seeded-in-template"

func migrateWidgets(_ context.Context, db *gorm.DB, dsn string) error {
	if dsn == "" {
		return gorm.ErrInvalidDB
	}
	if err := db.AutoMigrate(&Widget{}); err != nil {
		return err
	}
	return db.Create(&Widget{ID: seedWidget, Name: "template"}).Error
}

// adminDSN returns the server to run against: a postgres:// URL for a role that
// may CREATE DATABASE. The tests are skipped without one.
func adminDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func databaseNames(t *testing.T, dsn, like string) []string {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	var names []string
	require.NoError(t, db.Raw(
		`SELECT datname FROM pg_database WHERE datname LIKE ? ORDER BY datname`, like,
	).Scan(&names).Error)
	return names
}

func TestTemplatePoolFromDSN(t *testing.T) {
	dsn := adminDSN(t)
	// External-mode names start with this process's pid, which is how the test
	// tells this binary's databases from anything else on a shared server.
	ours := fmt.Sprintf("_%d_%%", os.Getpid())
	require.Empty(t, databaseNames(t, dsn, "gormx_template"+ours))

	// Two pools on one server at once, as two test binaries under go test -p 2
	// would be: each needs its own template and forks.
	first := gormx.NewTemplatePoolFromDSN(dsn, migrateWidgets)
	second := gormx.NewTemplatePoolFromDSN(dsn, migrateWidgets)

	t.Run("forks from both pools inherit the seed and stay isolated", func(t *testing.T) {
		a, b := first.Fork(t), second.Fork(t)

		require.NoError(t, a.Create(&Widget{ID: "only-in-first", Name: "a"}).Error)

		var count int64
		require.NoError(t, b.Model(&Widget{}).Where("id = ?", "only-in-first").Count(&count).Error)
		require.Zero(t, count, "a write in one fork must not be visible in another")
		require.NoError(t, b.Model(&Widget{}).Where("id = ?", seedWidget).Count(&count).Error)
		require.Equal(t, int64(1), count)

		require.Len(t, databaseNames(t, dsn, "gormx_template"+ours), 2, "each pool needs its own template")
		require.Len(t, databaseNames(t, dsn, "fork"+ours), 2, "one fork per pool")
	})

	t.Run("finished forks are dropped", func(t *testing.T) {
		// The subtest above has finished, so both of its forks must be gone.
		require.Empty(t, databaseNames(t, dsn, "fork"+ours))
	})

	t.Run("ForkDSN hands out a clone the caller opens itself", func(t *testing.T) {
		forkDSN := first.ForkDSN(t)

		// A plain gorm connection, none of gormx's plugins: the reason ForkDSN
		// exists.
		db, err := gorm.Open(postgres.Open(forkDSN), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })

		var got Widget
		require.NoError(t, db.Where("id = ?", seedWidget).First(&got).Error)
		require.Equal(t, "template", got.Name)
		require.Len(t, databaseNames(t, dsn, "fork"+ours), 1)
	})

	t.Run("a ForkDSN clone is dropped with its test", func(t *testing.T) {
		require.Empty(t, databaseNames(t, dsn, "fork"+ours))
	})

	t.Run("Close drops each pool's template", func(t *testing.T) {
		require.NoError(t, first.Close(context.Background()))
		require.NoError(t, second.Close(context.Background()))
		require.Empty(t, databaseNames(t, dsn, "gormx_template"+ours))
	})
}

func TestNewTemplatePoolFromDSN_RequiresArguments(t *testing.T) {
	require.PanicsWithValue(t, "gormx: NewTemplatePoolFromDSN requires a TemplateMigrator", func() {
		gormx.NewTemplatePoolFromDSN("postgres://localhost/postgres", nil)
	})
	require.PanicsWithValue(t, "gormx: NewTemplatePoolFromDSN requires an admin DSN", func() {
		gormx.NewTemplatePoolFromDSN("", migrateWidgets)
	})
}
