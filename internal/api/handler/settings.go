package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// PublicSettingKeys are the keys exposed without authentication.
var PublicSettingKeys = []string{"bulletin"}

// SettingsHandler serves the public subset of the `system` key/value store,
// which is the same shape Hydro's model/system.ts uses.
type SettingsHandler struct {
	db *mongo.Database
}

func NewSettingsHandler(db *mongo.Database) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/api/v1/settings", h.publicSettings)
}

func (h *SettingsHandler) publicSettings(c *gin.Context) {
	values, err := readSettings(c.Request.Context(), h.db, PublicSettingKeys)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to load settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": values})
}

// readSettings loads the given keys, returning an empty string for missing ones.
func readSettings(ctx context.Context, db *mongo.Database, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = ""
	}
	cursor, err := db.Collection("system").Find(ctx, bson.M{"_id": bson.M{"$in": keys}},
		options.Find().SetProjection(bson.M{"value": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Value any    `bson:"value"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		if text, ok := doc.Value.(string); ok {
			values[doc.ID] = text
		}
	}
	return values, cursor.Err()
}
