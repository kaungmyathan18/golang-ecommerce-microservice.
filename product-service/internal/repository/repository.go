package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrDuplicateName      = errors.New("duplicate name")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrCategoryInUse      = errors.New("category in use")
)

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CategoryID  string    `json:"category_id"`
	Stock       int       `json:"stock"`
	CreatedAt   time.Time `json:"created_at"`
}

type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository struct {
	products   *mongo.Collection
	categories *mongo.Collection
	log        *zap.Logger
}

func NewRepository(client *mongo.Client, dbName string, log *zap.Logger) *Repository {
	db := client.Database(dbName)
	return &Repository{
		products:   db.Collection("products"),
		categories: db.Collection("categories"),
		log:        log,
	}
}

type productDoc struct {
	ID          string    `bson:"_id"`
	Name        string    `bson:"name"`
	Description string    `bson:"description"`
	PriceCents  int64     `bson:"price_cents"`
	CategoryID  string    `bson:"category_id"`
	Stock       int       `bson:"stock"`
	CreatedAt   time.Time `bson:"created_at"`
}

type categoryDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	CreatedAt time.Time `bson:"created_at"`
}

func centsToFloat(c int64) float64 { return float64(c) / 100 }
func floatToCents(f float64) int64 { return int64(f * 100) }

func docToProduct(d productDoc) Product {
	return Product{
		ID: d.ID, Name: d.Name, Description: d.Description,
		Price: centsToFloat(d.PriceCents), CategoryID: d.CategoryID,
		Stock: d.Stock, CreatedAt: d.CreatedAt,
	}
}

func EnsureIndexes(ctx context.Context, client *mongo.Client, dbName string) error {
	db := client.Database(dbName)
	_, err := db.Collection("categories").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return err
	}
	_, err = db.Collection("products").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "category_id", Value: 1}},
	})
	return err
}

func (r *Repository) CreateCategory(ctx context.Context, name string) (*Category, error) {
	c := Category{ID: uuid.NewString(), Name: name, CreatedAt: time.Now().UTC()}
	_, err := r.categories.InsertOne(ctx, categoryDoc{ID: c.ID, Name: name, CreatedAt: c.CreatedAt})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDuplicateName
		}
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]Category, error) {
	cur, err := r.categories.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"created_at": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []Category
	for cur.Next(ctx) {
		var d categoryDoc
		if err := cur.Decode(&d); err != nil {
			return nil, err
		}
		out = append(out, Category{ID: d.ID, Name: d.Name, CreatedAt: d.CreatedAt})
	}
	return out, cur.Err()
}

func (r *Repository) UpdateCategory(ctx context.Context, id, name string) (*Category, error) {
	res := r.categories.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"name": name}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var d categoryDoc
	if err := res.Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDuplicateName
		}
		return nil, err
	}
	return &Category{ID: d.ID, Name: d.Name, CreatedAt: d.CreatedAt}, nil
}

func (r *Repository) DeleteCategory(ctx context.Context, id string) error {
	count, err := r.products.CountDocuments(ctx, bson.M{"category_id": id})
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrCategoryInUse
	}
	res, err := r.categories.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CreateProduct(ctx context.Context, name, description string, price float64, categoryID string, stock int) (*Product, error) {
	p := Product{
		ID: uuid.NewString(), Name: name, Description: description,
		Price: price, CategoryID: categoryID, Stock: stock,
		CreatedAt: time.Now().UTC(),
	}
	_, err := r.products.InsertOne(ctx, productDoc{
		ID: p.ID, Name: name, Description: description,
		PriceCents: floatToCents(price), CategoryID: categoryID,
		Stock: stock, CreatedAt: p.CreatedAt,
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) GetProduct(ctx context.Context, id string) (*Product, error) {
	var d productDoc
	err := r.products.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p := docToProduct(d)
	return &p, nil
}

func (r *Repository) ListProducts(ctx context.Context, offset, limit int) ([]Product, int64, error) {
	total, err := r.products.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSkip(int64(offset)).SetLimit(int64(limit)).SetSort(bson.M{"created_at": 1})
	cur, err := r.products.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []Product
	for cur.Next(ctx) {
		var d productDoc
		if err := cur.Decode(&d); err != nil {
			return nil, 0, err
		}
		out = append(out, docToProduct(d))
	}
	return out, total, cur.Err()
}

func (r *Repository) UpdateProduct(ctx context.Context, id, name, description string, price float64, categoryID string, stock int) (*Product, error) {
	res := r.products.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"name": name, "description": description,
			"price_cents": floatToCents(price), "category_id": categoryID, "stock": stock,
		}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var d productDoc
	if err := res.Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p := docToProduct(d)
	return &p, nil
}

func (r *Repository) DeleteProduct(ctx context.Context, id string) error {
	res, err := r.products.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DecrementStock(ctx context.Context, id string, qty int) (int, error) {
	res := r.products.FindOneAndUpdate(ctx,
		bson.M{"_id": id, "stock": bson.M{"$gte": qty}},
		bson.M{"$inc": bson.M{"stock": -qty}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var d productDoc
	if err := res.Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, ErrInsufficientStock
		}
		return 0, err
	}
	return d.Stock, nil
}

func (r *Repository) IncrementStock(ctx context.Context, id string, qty int) (int, error) {
	res := r.products.FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$inc": bson.M{"stock": qty}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var d productDoc
	if err := res.Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return d.Stock, nil
}
