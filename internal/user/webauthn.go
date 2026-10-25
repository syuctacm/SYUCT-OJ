package user

import (
	"context"
	"crypto/rand"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// EnsureWebAuthnID returns the stable opaque user handle used by WebAuthn.
func (r *Repository) EnsureWebAuthnID(ctx context.Context, uid int64) ([]byte, error) {
	account, err := r.FindByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if len(account.WebAuthnID) > 0 {
		return append([]byte(nil), account.WebAuthnID...), nil
	}
	handle := make([]byte, 32)
	if _, err := rand.Read(handle); err != nil {
		return nil, err
	}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid, "webauthnId": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"webauthnId": handle}})
	if err != nil {
		return nil, err
	}
	if result.ModifiedCount == 0 {
		account, err = r.FindByID(ctx, uid)
		if err != nil {
			return nil, err
		}
		if len(account.WebAuthnID) > 0 {
			return account.WebAuthnID, nil
		}
		return nil, fmt.Errorf("unable to initialize WebAuthn user handle")
	}
	return handle, nil
}
