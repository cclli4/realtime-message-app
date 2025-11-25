package repository

import (
	"context"
	"fmt"
	"github.com/cclli4/realtime-message-app/app/models"
	"github.com/cclli4/realtime-message-app/pkg/database"
	"go.elastic.co/apm"
	"go.mongodb.org/mongo-driver/bson"
)

func InsertNewMessage(ctx context.Context, data models.MessagePayload) error {
	span, _ := apm.StartSpan(ctx, "InsertNewMessage", "repository")
	defer span.End()

	if database.MongoDB == nil {
		return fmt.Errorf("MongoDB collection is not initialized")
	}

	_, err := database.MongoDB.InsertOne(ctx, data)
	return err
}

func GetAllMessage(ctx context.Context) ([]models.MessagePayload, error) {
	span, _ := apm.StartSpan(ctx, "GetAllMessage", "repository")
	defer span.End()

	var (
		err  error
		resp []models.MessagePayload
	)
	
	if database.MongoDB == nil {
		return resp, fmt.Errorf("MongoDB collection is not initialized")
	}
	
	cursor, err := database.MongoDB.Find(ctx, bson.D{})
	if err != nil {
		return resp, fmt.Errorf("failed to find message: %v", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		payload := models.MessagePayload{}
		if err := cursor.Decode(&payload); err != nil {
			return resp, fmt.Errorf("failed to decode message: %v", err)
		}
		resp = append(resp, payload)
	}
	
	if err := cursor.Err(); err != nil {
		return resp, fmt.Errorf("cursor error: %v", err)
	}
	
	return resp, nil
}
