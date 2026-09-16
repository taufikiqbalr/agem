package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserRow struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type userDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Email     string             `bson:"email,omitempty"`
	Phone     string             `bson:"phone,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

func (s *Store) CreateUser(ctx context.Context, email, phone string) (*UserRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	doc := userDoc{
		ID:        primitive.NewObjectID(),
		Email:     email,
		Phone:     phone,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if _, err := s.db.Collection("users").InsertOne(cctx, doc); err != nil {
		return nil, err
	}
	return userDocToRow(doc), nil
}

func (s *Store) GetUser(ctx context.Context, id string) (*UserRow, error) {
	if s.externalUserDB {
		user, err := s.getUserFromCollection(ctx, s.usersCollection(), id)
		if err == nil {
			return user, nil
		}
		if err != ErrNotFound {
			return nil, err
		}
	}
	return s.getUserFromCollection(ctx, s.db.Collection("users"), id)
}

func (s *Store) UserExists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	_, err := s.GetUser(ctx, id)
	if err == nil {
		return true, nil
	}
	if err == ErrNotFound {
		return false, nil
	}
	return false, err
}

func (s *Store) getUserFromCollection(ctx context.Context, col *mongo.Collection, id string) (*UserRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	var doc bson.M
	if err := col.FindOne(cctx, userIDFilter(id)).Decode(&doc); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return userMapToRow(doc), nil
}

func (s *Store) UpdateUser(ctx context.Context, id string, email, phone *string) (*UserRow, error) {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return nil, err
	}

	set := bson.M{"updated_at": time.Now().UTC()}
	if email != nil {
		set["email"] = *email
	}
	if phone != nil {
		set["phone"] = *phone
	}

	var doc userDoc
	err = s.db.Collection("users").FindOneAndUpdate(
		cctx,
		bson.M{"_id": oid},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return userDocToRow(doc), nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	cctx, cancel := ctxTimeout(ctx)
	defer cancel()

	oid, err := objectIDFromString(id)
	if err != nil {
		return err
	}

	res, err := s.db.Collection("users").DeleteOne(cctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func userDocToRow(doc userDoc) *UserRow {
	return &UserRow{
		ID:        objectIDString(doc.ID),
		Email:     doc.Email,
		Phone:     doc.Phone,
		CreatedAt: doc.CreatedAt.Format(time.RFC3339),
		UpdatedAt: doc.UpdatedAt.Format(time.RFC3339),
	}
}

func userIDFilter(id string) bson.M {
	filters := []bson.M{
		{"_id": id},
		{"id": id},
		{"user_id": id},
	}
	if oid, err := objectIDFromString(id); err == nil {
		filters = append([]bson.M{{"_id": oid}}, filters...)
	}
	return bson.M{"$or": filters}
}

func userMapToRow(doc bson.M) *UserRow {
	return &UserRow{
		ID:        userIDString(doc["_id"]),
		Email:     firstStringField(doc, "email", "email_address"),
		Phone:     firstStringField(doc, "phone", "phone_number", "mobile"),
		CreatedAt: firstTimeField(doc, "created_at", "createdAt", "created"),
		UpdatedAt: firstTimeField(doc, "updated_at", "updatedAt", "updated"),
	}
}

func userIDString(value any) string {
	switch v := value.(type) {
	case primitive.ObjectID:
		return v.Hex()
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func firstStringField(doc bson.M, keys ...string) string {
	for _, key := range keys {
		value, ok := doc[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

func firstTimeField(doc bson.M, keys ...string) string {
	for _, key := range keys {
		value, ok := doc[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case time.Time:
			if !v.IsZero() {
				return v.UTC().Format(time.RFC3339)
			}
		case primitive.DateTime:
			return v.Time().UTC().Format(time.RFC3339)
		case string:
			return v
		}
	}
	return ""
}
