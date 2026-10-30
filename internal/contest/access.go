package contest

import "go.mongodb.org/mongo-driver/v2/bson"

// HasAssignAccess mirrors Hydro's contest assignment check. Owners,
// maintainers and privileged users bypass the group restriction.
func HasAssignAccess(item *Contest, uid int64, groups []string, privileged bool) bool {
	if item == nil {
		return false
	}
	if len(item.Assign) == 0 || privileged || (uid > 0 && item.Owner == uid) {
		return true
	}
	if uid > 0 && containsUID(item.Maintainer, uid) {
		return true
	}
	joined := make(map[string]bool, len(groups))
	for _, group := range groups {
		joined[group] = true
	}
	for _, group := range item.Assign {
		if joined[group] {
			return true
		}
	}
	return false
}

// AssignFilter returns a MongoDB filter equivalent to HasAssignAccess.
func AssignFilter(uid int64, groups []string, privileged bool) bson.M {
	if privileged {
		return bson.M{}
	}
	or := []bson.M{
		{"assign": bson.M{"$exists": false}},
		{"assign": nil},
		{"assign": bson.M{"$size": 0}},
	}
	if uid > 0 {
		or = append(or, bson.M{"owner": uid}, bson.M{"maintainer": uid})
	}
	if len(groups) > 0 {
		or = append(or, bson.M{"assign": bson.M{"$in": groups}})
	}
	return bson.M{"$or": or}
}

func containsUID(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
