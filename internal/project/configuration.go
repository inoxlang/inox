package project

type ProjectConfiguration struct {
	ExposeWebServers bool
}

func (c ProjectConfiguration) AreExposedWebServersAllowed() bool {
	return c.ExposeWebServers
}
