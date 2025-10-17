package repository

import "bot_story_generator/internal/database"

type StoryDatabaseImpl struct {
	databaseclient *database.DBObject
}

func NewStoryDatabase(db *database.DBObject) *StoryDatabaseImpl {
	return &StoryDatabaseImpl{databaseclient: db}
}

//методы базы данных
