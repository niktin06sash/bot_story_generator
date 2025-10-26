CREATE TABLE storiesVariants(
    storyID INT NOT NULL PRIMARY KEY,
    data    JSONB NOT NULL,
    type    TEXT NOT NULL,
    CONSTRAINT fk_story
        FOREIGN KEY (storyID)
        REFERENCES stories(ID)
        ON DELETE CASCADE
);