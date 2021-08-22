package db

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Rating struct {
	ID             string           `bson:"_id"`
	Rating         float64          `bson:"rating"`
	RatingsHistory []*RatingHistory `bson:"rating_history"`
}

type RatingHistory struct {
	Date   primitive.Timestamp `bson:"date"`
	Rating float64             `bson:"rating"`
}

var ColRatings string = "ratings"

func (c *MongoDBClient) GetRatingsForPlayers(ids []string) ([]*Rating, error) {
	var res []*Rating
	cursor, err := c.Client.Database(c.Database).Collection(ColRatings).Find(
		context.TODO(),
		bson.M{"_id": bson.M{"$in": ids}},
	)
	if err != nil {
		return nil, err
	}

	err = cursor.All(context.TODO(), &res)
	return res, err
}

func (c *MongoDBClient) UpsertRating(id string, rating float64, date primitive.Timestamp) error {
	_, err := c.Client.Database(c.Database).Collection(ColRatings).UpdateByID(
		context.TODO(),
		id,
		bson.M{"$set": bson.M{"rating": rating}, "$push": bson.M{"rating_history": RatingHistory{
			Date:   date,
			Rating: rating,
		}}},
		options.Update().SetUpsert(true),
	)

	return err
}

func (c *MongoDBClient) GetMissingPlayerInfos() ([]string, error) {
	lookupStage := bson.D{{
		"$lookup", bson.M{
			"localField":   "_id",
			"foreignField": "_id",
			"as":           "info",
			"from":         ColPlayers,
		},
	}}
	matchStage := bson.D{{
		"$match", bson.M{"info.0": bson.M{"$exists": false}},
	}}
	cursor, err := c.Client.Database(c.Database).Collection(ColRatings).Aggregate(context.TODO(),
		mongo.Pipeline{
			lookupStage, matchStage,
		})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	ids := make([]string, 0)
	var current Rating
	for cursor.Next(context.TODO()) {
		if cursor.Decode(&current) != nil {
			return nil, err
		}

		if current.ID == "" {
			continue
		}

		ids = append(ids, current.ID)
	}

	return ids, nil
}
