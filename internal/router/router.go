package router

type StoryService interface {
}
type StoryRouterImpl struct {
	service StoryService
}

func NewRouter(service StoryService) *StoryRouterImpl {
	return &StoryRouterImpl{
		service: service,
	}
}
