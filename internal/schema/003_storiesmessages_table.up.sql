CREATE TABLE storiesMessages (
    ID         SERIAL PRIMARY KEY,
    storyID    INT NOT NULL,
    data       TEXT NOT NULL,
    createdAt  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT fk_story
        FOREIGN KEY (storyID)
        REFERENCES stories(ID)
        ON DELETE CASCADE

);
CREATE INDEX idx_storiesMessages_storyID ON storiesMessages (storyID);