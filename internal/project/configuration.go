package project

type ProjectConfiguration struct {
	exposeWebServers bool
}

func (c ProjectConfiguration) AreExposedWebServersAllowed() bool {
	return c.exposeWebServers
}
