package main

import (
	"context"
	"fmt"
	"scoreinflux/db"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println("cannot connect to mongodb", err)
		return
	}

	cursor, err := client.Database("xstats").Collection("games").Find(context.TODO(), bson.M{})
	if err != nil {
		fmt.Println("cannot get games", err)
		return
	}
	defer cursor.Close(context.TODO())

	var current db.Game
	for cursor.Next(context.TODO()) {
		if cursor.Decode(&current) != nil {
			fmt.Println("cursor error", err)
			return
		}

		split := strings.Split(current.ID, "/")
		if len(split) < 7 {
			fmt.Println("weird game url " + current.ID)
			return
		}

		year, err := strconv.Atoi(split[4])
		if err != nil {
			fmt.Println("wrong year for " + current.ID)
		}

		month, err := strconv.Atoi(split[5])
		if err != nil {
			fmt.Println("wrong month for " + current.ID)
		}

		day, err := strconv.Atoi(split[6])
		if err != nil {
			fmt.Println("wrong day for " + current.ID)
		}

		t := primitive.Timestamp{
			T: uint32(time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Unix())}

		_, err = client.Database("xstats").Collection("games").UpdateOne(
			context.TODO(),
			bson.M{"_id": current.ID},
			bson.M{"$set": bson.M{"date": t}},
		)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}
