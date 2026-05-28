package mcp

type Server struct {
	Registry *Registry
}

func NewServer(deps Dependencies) *Server {
	return &Server{Registry: NewRegistry(deps)}
}
