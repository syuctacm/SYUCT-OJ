package user

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	PrivEditSystem  int64 = 1 << 0
	PrivUserProfile int64 = 1 << 2
)

// IsBanned reports the explicit moderation flag. The profile bit remains in
// sync for Hydro-compatible consumers, but the explicit flag avoids treating
// legacy test/fixture accounts with priv=0 as banned by accident.
func IsBanned(account *User) bool {
	return account != nil && account.Banned
}

// SetBanned toggles the user profile bit while preserving all other privileges.
func (r *Repository) SetBanned(ctx context.Context, uid int64, banned bool) (int64, error) {
	account, err := r.FindByID(ctx, uid)
	if err != nil {
		return 0, err
	}
	priv := account.Priv
	if banned {
		priv &^= PrivUserProfile
	} else {
		priv |= PrivUserProfile
	}
	result, err := r.collection.UpdateOne(ctx, bson.M{"_id": uid}, bson.M{"$set": bson.M{"priv": priv, "banned": banned}})
	if err != nil {
		return 0, err
	}
	if result.MatchedCount == 0 {
		return 0, errors.New("user not found")
	}
	return priv, nil
}
