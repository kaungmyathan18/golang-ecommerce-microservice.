package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Item struct {
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repository struct {
	coll *mongo.Collection
	log  *zap.Logger
}

func NewRepository(client *mongo.Client, dbName string, log *zap.Logger) *Repository {
	return &Repository{
		coll: client.Database(dbName).Collection("inventory"),
		log:  log,
	}
}

type itemDoc struct {
	ID        string    `bson:"_id"`
	Quantity  int       `bson:"quantity"`
	UpdatedAt time.Time `bson:"updated_at"`
}

func (r *Repository) GetStock(ctx context.Context, productID string) (int, error) {
	var d itemDoc
	err := r.coll.FindOne(ctx, bson.M{"_id": productID}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return d.Quantity, nil
}

func (r *Repository) SetStock(ctx context.Context, productID string, qty int) (int, error) {
	now := time.Now().UTC()
	opts := options.Update().SetUpsert(true)
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": productID},
		bson.M{"$set": bson.M{"quantity": qty, "updated_at": now}},
		opts,
	)
	if err != nil {
		return 0, err
	}
	return qty, nil
}

func (r *Repository) DecrementStock(ctx context.Context, productID string, qty int) (int, error) {
	res := r.coll.FindOneAndUpdate(ctx,
		bson.M{"_id": productID, "quantity": bson.M{"$gte": qty}},
		bson.M{"$inc": bson.M{"quantity": -qty}, "$set": bson.M{"updated_at": time.Now().UTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var d itemDoc
	if err := res.Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, ErrInsufficientStock
		}
		return 0, err
	}
	return d.Quantity, nil
}

func (r *Repository) IncrementStock(ctx context.Context, productID string, qty int) (int, error) {
	res := r.coll.FindOneAndUpdate(ctx,
		bson.M{"_id": productID},
		bson.M{"$inc": bson.M{"quantity": qty}, "$set": bson.M{"updated_at": time.Now().UTC()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true),
	)
	var d itemDoc
	if err := res.Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return d.Quantity, nil
}
