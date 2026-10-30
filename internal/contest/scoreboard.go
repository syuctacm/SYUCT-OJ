package contest

import (
	"math"
	"sort"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type scoreboardSubtask struct {
	Status int     `bson:"status"`
	Score  float64 `bson:"score"`
}

type scoreboardRecord struct {
	ID       bson.ObjectID                `bson:"_id"`
	UID      int64                        `bson:"uid"`
	PID      int64                        `bson:"pid"`
	Status   int                          `bson:"status"`
	Score    float64                      `bson:"score"`
	Subtasks map[string]scoreboardSubtask `bson:"subtasks"`
}

type scoreboardAttendee struct {
	UID    int64
	Unrank bool
}

type scoreboardState struct {
	uid          int64
	solved       int
	penalty      int
	score        float64
	original     float64
	time         int
	cells        map[string]ProblemCell
	wrong        map[int64]int
	subtaskBest  map[int64]map[string]scoreboardSubtask
	subtaskRID   map[int64]string
	subtaskFirst map[int64]bool
}

func newScoreboardState(uid int64) *scoreboardState {
	return &scoreboardState{
		uid:          uid,
		cells:        make(map[string]ProblemCell),
		wrong:        make(map[int64]int),
		subtaskBest:  make(map[int64]map[string]scoreboardSubtask),
		subtaskRID:   make(map[int64]string),
		subtaskFirst: make(map[int64]bool),
	}
}

func problemWeight(item *Contest, pid int64) float64 {
	weight := item.Score[pid]
	if weight <= 0 {
		weight = 100
	}
	return float64(weight) / 100
}

func firstAccepted(item *Contest, records []scoreboardRecord) map[int64]int64 {
	first := make(map[int64]int64)
	for _, record := range records {
		if record.Status != 1 {
			continue
		}
		nanos := record.ID.Timestamp().UnixNano()
		if current, ok := first[record.PID]; !ok || nanos < current {
			first[record.PID] = nanos
		}
	}
	return first
}

func markFirstAccepts(rows []ScoreboardRow, first map[int64]int64) {
	for i := range rows {
		for key, cell := range rows[i].Problems {
			if !cell.Solved || cell.RID == "" {
				continue
			}
			oid, err := bson.ObjectIDFromHex(cell.RID)
			if err != nil {
				continue
			}
			pid, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				continue
			}
			if firstAt, ok := first[pid]; ok && oid.Timestamp().UnixNano() == firstAt {
				cell.First = true
				rows[i].Problems[key] = cell
			}
		}
	}
}

func pendingForRecord(item *Contest, record scoreboardRecord) bool {
	return item.LockAt != nil && !item.Unlocked && record.ID.Timestamp().After(*item.LockAt)
}

func buildScoreboard(item *Contest, attendees []scoreboardAttendee, records []scoreboardRecord) []ScoreboardRow {
	users := make(map[int64]*scoreboardState)
	unranked := make(map[int64]bool)
	for _, attendee := range attendees {
		if attendee.Unrank {
			unranked[attendee.UID] = true
			continue
		}
		if attendee.UID > 0 {
			users[attendee.UID] = newScoreboardState(attendee.UID)
		}
	}
	first := firstAccepted(item, records)
	for _, record := range records {
		if record.UID <= 0 || unranked[record.UID] {
			continue
		}
		allowed := false
		for _, pid := range item.PIDs {
			if pid == record.PID {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		state := users[record.UID]
		if state == nil {
			state = newScoreboardState(record.UID)
			users[record.UID] = state
		}
		switch item.Rule {
		case "oi", "ioi":
			applyScoreboardOI(item, state, record)
		case "strictioi":
			applyScoreboardStrictIOI(item, state, record, first)
		case "homework":
			applyScoreboardHomework(item, state, record)
		case "ledo":
			applyScoreboardLedo(item, state, record)
		default:
			applyScoreboardACM(item, state, record)
		}
	}
	rows := make([]ScoreboardRow, 0, len(users))
	for _, state := range users {
		rows = append(rows, ScoreboardRow{
			UID: state.uid, Solved: state.solved, Penalty: state.penalty,
			Score: state.score, OriginalScore: state.original, Time: state.time,
			Problems: state.cells,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		switch item.Rule {
		case "oi", "ioi", "strictioi":
			if rows[i].Score != rows[j].Score {
				return rows[i].Score > rows[j].Score
			}
		case "homework":
			if rows[i].Score != rows[j].Score {
				return rows[i].Score > rows[j].Score
			}
			if rows[i].Time != rows[j].Time {
				return rows[i].Time < rows[j].Time
			}
		case "ledo":
			if rows[i].Score != rows[j].Score {
				return rows[i].Score > rows[j].Score
			}
		default:
			if rows[i].Solved != rows[j].Solved {
				return rows[i].Solved > rows[j].Solved
			}
			if rows[i].Penalty != rows[j].Penalty {
				return rows[i].Penalty < rows[j].Penalty
			}
		}
		return rows[i].UID < rows[j].UID
	})
	for i := range rows {
		if i == 0 {
			rows[i].Rank = 1
			continue
		}
		same := false
		switch item.Rule {
		case "oi", "ioi", "strictioi", "ledo":
			same = rows[i].Score == rows[i-1].Score
		case "homework":
			same = rows[i].Score == rows[i-1].Score && rows[i].Time == rows[i-1].Time
		default:
			same = rows[i].Solved == rows[i-1].Solved && rows[i].Penalty == rows[i-1].Penalty
		}
		if same {
			rows[i].Rank = rows[i-1].Rank
		} else {
			rows[i].Rank = i + 1
		}
	}
	markFirstAccepts(rows, first)
	return rows
}

func applyScoreboardACM(item *Contest, state *scoreboardState, record scoreboardRecord) {
	key := strconv.FormatInt(record.PID, 10)
	cell := state.cells[key]
	if cell.Solved {
		return
	}
	if pendingForRecord(item, record) {
		cell.Pending++
		state.cells[key] = cell
		return
	}
	if record.Status == 1 {
		minutes := int(record.ID.Timestamp().Sub(item.BeginAt).Minutes())
		if minutes < 0 {
			minutes = 0
		}
		cell.Solved = true
		cell.Attempts = state.wrong[record.PID]
		cell.Time = minutes
		cell.RID = record.ID.Hex()
		cell.Score = 100
		state.cells[key] = cell
		state.solved++
		state.penalty += minutes + 20*state.wrong[record.PID]
		return
	}
	if record.Status >= 2 && record.Status <= 6 {
		state.wrong[record.PID]++
		cell.Attempts = state.wrong[record.PID]
		cell.RID = record.ID.Hex()
		state.cells[key] = cell
	}
}

func applyScoreboardOI(item *Contest, state *scoreboardState, record scoreboardRecord) {
	key := strconv.FormatInt(record.PID, 10)
	cell := state.cells[key]
	if record.Status != 1 {
		cell.Attempts++
	}
	if pendingForRecord(item, record) {
		cell.Pending++
		state.cells[key] = cell
		return
	}
	if record.Score > cell.Score || cell.RID == "" {
		state.score -= cell.Score * problemWeight(item, record.PID)
		cell.Score = record.Score
		cell.Solved = record.Status == 1
		cell.Time = int(record.ID.Timestamp().Sub(item.BeginAt).Seconds())
		cell.RID = record.ID.Hex()
		state.score += cell.Score * problemWeight(item, record.PID)
	}
	state.cells[key] = cell
	state.original = state.score
}

func applyScoreboardStrictIOI(item *Contest, state *scoreboardState, record scoreboardRecord, first map[int64]int64) {
	key := strconv.FormatInt(record.PID, 10)
	cell := state.cells[key]
	cell.Attempts++
	best := state.subtaskBest[record.PID]
	if best == nil {
		best = make(map[string]scoreboardSubtask)
		state.subtaskBest[record.PID] = best
	}
	changed := false
	for id, result := range record.Subtasks {
		current, ok := best[id]
		if !ok || result.Score > current.Score || (result.Score == current.Score && result.Status > current.Status) {
			best[id] = result
			changed = true
		}
	}
	if len(record.Subtasks) == 0 && record.Score > cell.Score {
		cell.Score = record.Score
		changed = true
	}
	if !changed && cell.RID != "" {
		state.cells[key] = cell
		return
	}
	if len(record.Subtasks) > 0 {
		total := 0.0
		maxStatus := 0
		for _, result := range best {
			total += result.Score
			if result.Status > maxStatus {
				maxStatus = result.Status
			}
		}
		cell.Score = total
		cell.Solved = maxStatus == 1
	} else if record.Status == 1 {
		cell.Solved = true
	}
	cell.Time = int(record.ID.Timestamp().Sub(item.BeginAt).Seconds())
	cell.RID = record.ID.Hex()
	state.subtaskRID[record.PID] = cell.RID
	state.cells[key] = cell
	state.score = 0
	for pid, value := range state.cells {
		parsed, _ := strconv.ParseInt(pid, 10, 64)
		state.score += value.Score * problemWeight(item, parsed)
	}
	state.original = state.score
	if acceptedAt, ok := first[record.PID]; ok && cell.Solved && record.ID.Timestamp().UnixNano() == acceptedAt {
		state.subtaskFirst[record.PID] = true
	}
}

func homeworkPenaltyCoefficient(item *Contest, submittedAt int64) float64 {
	if item.PenaltySince == nil || submittedAt < item.PenaltySince.UnixMilli() {
		return 1
	}
	exceedSeconds := float64(submittedAt-item.PenaltySince.UnixMilli()) / 1000
	keys := make([]float64, 0, len(item.PenaltyRules))
	for key := range item.PenaltyRules {
		value, err := strconv.ParseFloat(key, 64)
		if err == nil {
			keys = append(keys, value)
		}
	}
	sort.Float64s(keys)
	coefficient := 1.0
	for _, hour := range keys {
		if hour*3600 <= exceedSeconds {
			coefficient = item.PenaltyRules[strconv.FormatFloat(hour, 'f', -1, 64)]
		} else {
			break
		}
	}
	return coefficient
}

func applyScoreboardHomework(item *Contest, state *scoreboardState, record scoreboardRecord) {
	key := strconv.FormatInt(record.PID, 10)
	cell := state.cells[key]
	cell.Attempts++
	submittedAt := record.ID.Timestamp()
	cell.Score = record.Score
	cell.PenaltyScore = record.Score * homeworkPenaltyCoefficient(item, submittedAt.UnixMilli())
	cell.Solved = record.Status == 1
	cell.Time = int(submittedAt.Sub(item.BeginAt).Seconds())
	cell.RID = record.ID.Hex()
	state.cells[key] = cell
	state.score = 0
	state.original = 0
	state.time = 0
	for pid, value := range state.cells {
		parsed, _ := strconv.ParseInt(pid, 10, 64)
		weight := problemWeight(item, parsed)
		state.score += value.PenaltyScore * weight
		state.original += value.Score * weight
		state.time += value.Time
	}
}

func applyScoreboardLedo(item *Contest, state *scoreboardState, record scoreboardRecord) {
	key := strconv.FormatInt(record.PID, 10)
	cell := state.cells[key]
	if record.Status == 7 || record.Status == 8 || record.Status == 9 {
		state.cells[key] = cell
		return
	}
	state.wrong[record.PID]++
	attempts := state.wrong[record.PID]
	coefficient := math.Max(0.7, math.Pow(0.95, float64(attempts-1)))
	penaltyScore := math.Round(coefficient * record.Score)
	cell.Attempts = attempts - 1
	if cell.RID == "" || penaltyScore > cell.PenaltyScore {
		cell.Score = record.Score
		cell.PenaltyScore = penaltyScore
		cell.Solved = record.Status == 1
		cell.Time = int(record.ID.Timestamp().Sub(item.BeginAt).Seconds())
		cell.RID = record.ID.Hex()
	}
	state.cells[key] = cell
	state.score = 0
	state.original = 0
	state.time = 0
	for pid, value := range state.cells {
		parsed, _ := strconv.ParseInt(pid, 10, 64)
		weight := problemWeight(item, parsed)
		state.score += value.PenaltyScore * weight
		state.original += value.Score * weight
		state.time += value.Time
	}
}
