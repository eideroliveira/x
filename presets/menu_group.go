package presets

type MenuGroupBuilder struct {
	name string
	icon string
	// item can be URI name, model name
	// the underlying logic is using URI name,
	// so if the URI name is customized, item must be the URI name
	subMenuItems []string
}

func (b *MenuGroupBuilder) Icon(v string) (r *MenuGroupBuilder) {
	b.icon = v
	return b
}

func (b *MenuGroupBuilder) SubItems(ss ...string) (r *MenuGroupBuilder) {
	b.subMenuItems = ss
	return b
}

type MenuGroups struct {
	menuGroups []*MenuGroupBuilder
}

func (g *MenuGroups) MenuGroup(name string) (r *MenuGroupBuilder) {
	for _, mg := range g.menuGroups {
		if mg.name == name {
			return mg
		}
	}
	r = &MenuGroupBuilder{name: name}
	g.menuGroups = append(g.menuGroups, r)
	return
}
