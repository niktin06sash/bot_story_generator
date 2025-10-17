package service

type StoryDatabase interface {
	//методы для базы данных(пакет repository)
}
type StoryAI interface {
	GetChatCompletion(message_history string) (string, error)
}

type StoryServiceImpl struct {
	DBStory StoryDatabase
	AIStory StoryAI
}

func NewStoryService(db StoryDatabase, ai StoryAI) *StoryServiceImpl {
	return &StoryServiceImpl{DBStory: db, AIStory: ai}
}
