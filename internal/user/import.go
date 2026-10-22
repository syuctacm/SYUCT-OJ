package user

// Bulk account creation for onsite contests, ported from Hydro's
// SystemUserImportHandler. Each line is tab- or comma-separated:
//
//	email, username, password, displayName, extraJSON
//
// extraJSON may carry {"school": "...", "studentId": "...", "group": "..."}.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ImportUser struct {
	Email       string
	Username    string
	Password    string
	DisplayName string
	School      string
	StudentID   string
	Group       string
}

// ValidEmail matches Hydro's Types.Email check.
func ValidEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(address.Address, value) && strings.Contains(value, "@")
}

// ValidUsername mirrors Hydro's Types.Username: 3..31 characters, or exactly
// two Chinese characters.
func ValidUsername(value string) bool {
	count := utf8.RuneCountInString(value)
	if count >= 3 && count <= 31 {
		return true
	}
	if count != 2 {
		return false
	}
	for _, char := range value {
		if !unicode.Is(unicode.Han, char) {
			return false
		}
	}
	return true
}

// ValidPassword mirrors Hydro's Types.Password (6-255 bytes).
func ValidPassword(value string) bool {
	size := len([]byte(value))
	return size >= 6 && size <= 255
}

// ParseImportUsers parses the admin import textarea and returns the valid rows
// plus per-line messages.
func ParseImportUsers(raw string) ([]ImportUser, []string) {
	users := make([]ImportUser, 0)
	messages := make([]string, 0)
	seenEmail := map[string]bool{}
	seenName := map[string]bool{}
	for index, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := splitImportLine(line)
		if len(fields) < 3 || fields[0] == "" || fields[1] == "" || fields[2] == "" {
			messages = append(messages, fmt.Sprintf("第 %d 行：格式无效（需要邮箱、用户名、密码）", index+1))
			continue
		}
		input := ImportUser{Email: fields[0], Username: fields[1], Password: fields[2]}
		if len(fields) > 3 {
			input.DisplayName = fields[3]
		}
		if len(fields) > 4 {
			input.applyExtra(strings.Join(fields[4:], ","))
		}
		switch {
		case !ValidEmail(input.Email):
			messages = append(messages, fmt.Sprintf("第 %d 行：邮箱无效", index+1))
			continue
		case !ValidUsername(input.Username):
			messages = append(messages, fmt.Sprintf("第 %d 行：用户名需为 3-31 个字符（或 2 个汉字）", index+1))
			continue
		case !ValidPassword(input.Password):
			messages = append(messages, fmt.Sprintf("第 %d 行：密码长度需为 6-255 个字符", index+1))
			continue
		}
		emailKey, nameKey := strings.ToLower(input.Email), strings.ToLower(input.Username)
		if seenEmail[emailKey] || seenName[nameKey] {
			messages = append(messages, fmt.Sprintf("第 %d 行：邮箱或用户名在本次导入中重复", index+1))
			continue
		}
		seenEmail[emailKey] = true
		seenName[nameKey] = true
		users = append(users, input)
	}
	return users, messages
}

func splitImportLine(line string) []string {
	separator := ","
	if strings.Contains(line, "\t") {
		separator = "\t"
	}
	parts := strings.Split(line, separator)
	fields := make([]string, len(parts))
	for i, part := range parts {
		fields[i] = strings.TrimSpace(part)
	}
	return fields
}

func (u *ImportUser) applyExtra(extra string) {
	if strings.TrimSpace(extra) == "" {
		return
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(extra), &data); err != nil {
		return
	}
	if value, ok := data["school"].(string); ok {
		u.School = value
	}
	if value, ok := data["studentId"].(string); ok {
		u.StudentID = value
	}
	if value, ok := data["group"].(string); ok {
		u.Group = value
	}
}

// ImportUsers creates every parsed account, collecting per-row errors.
func (r *Repository) ImportUsers(ctx context.Context, users []ImportUser, ip string) ([]int64, []string, error) {
	uids := make([]int64, 0, len(users))
	messages := make([]string, 0)
	for _, input := range users {
		account, err := r.Create(ctx, RegisterInput{
			Mail: input.Email, Username: input.Username, Password: input.Password, IP: ip,
		})
		if err != nil {
			messages = append(messages, fmt.Sprintf("%s：%v", input.Email, err))
			continue
		}
		set := bson.M{}
		if input.DisplayName != "" {
			set["displayName"] = input.DisplayName
		}
		if input.School != "" {
			set["school"] = input.School
		}
		if input.StudentID != "" {
			set["studentId"] = input.StudentID
		}
		if len(set) > 0 {
			if err := r.UpdateSettings(ctx, account.ID, set); err != nil {
				messages = append(messages, fmt.Sprintf("%s：资料写入失败：%v", input.Email, err))
			}
		}
		if input.Group != "" {
			if err := r.AddGroupMembers(ctx, input.Group, []int64{account.ID}); err != nil {
				messages = append(messages, fmt.Sprintf("%s：分组写入失败：%v", input.Email, err))
			}
		}
		uids = append(uids, account.ID)
	}
	return uids, messages, nil
}

// AddGroupMembers appends users to a named group, creating it on demand.
func (r *Repository) AddGroupMembers(ctx context.Context, name string, uids []int64) error {
	if strings.TrimSpace(name) == "" || len(uids) == 0 {
		return nil
	}
	_, err := r.groups.UpdateOne(ctx,
		bson.M{"domainId": "system", "name": name},
		bson.M{"$addToSet": bson.M{"uids": bson.M{"$each": uids}}},
		options.UpdateOne().SetUpsert(true))
	return err
}
