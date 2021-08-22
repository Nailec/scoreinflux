package db

import "context"

type PlayerInfo struct {
	ID          string `bson:"_id"`
	DateOfBirth string `bson:"date_of_birth"`
	FirstName   string `bson:"first_name"`
	LastName    string `bson:"last_name"`
	Nationality string `bson:"nationality"`
}

var ColPlayers string = "players"

func (c *MongoDBClient) InsertPlayerInfo(pi *PlayerInfo) error {
	_, err := c.Client.Database(c.Database).Collection(ColPlayers).InsertOne(
		context.TODO(),
		pi,
	)

	return err
}
