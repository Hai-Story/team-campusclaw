package curriculum

import (
	"testing"
	"time"
)

func TestReviewCalendarAndLevels(t *testing.T) {
	// R2: the server's Chinese calendar, including midnight and month/year boundaries.
	now := time.Date(2026, 12, 31, 23, 59, 59, 0, learningZone)
	ints := []int{1, 3, 7, 14, 30}
	for level := 0; level <= 5; level++ {
		n, d, e := reviewSchedule(now, level, "good", ints)
		want := level + 1
		if want > 5 {
			want = 5
		}
		if e != nil || n != want || d.In(learningZone).Hour() != 9 || !d.Equal(time.Date(2026, 12, 31, 9, 0, 0, 0, learningZone).AddDate(0, 0, ints[want-1])) {
			t.Fatal(level, n, d, e)
		}
		n, d, e = reviewSchedule(now, level, "again", ints)
		if e != nil || n != 0 || d.In(learningZone).Format("2006-01-02 15:04") != "2027-01-01 09:00" {
			t.Fatal(n, d, e)
		}
	}
	n, d, e := reviewSchedule(now.Add(time.Second), 0, "good", ints)
	if e != nil || n != 1 || d.In(learningZone).Format("2006-01-02 15:04") != "2027-01-02 09:00" {
		t.Fatal(n, d, e)
	}
	for _, v := range [][]int{nil, {1, 3, 7, 14}, {1, 3, 3, 14, 30}, {0, 3, 7, 14, 30}, {1, 3, 7, 14, 366}} {
		if validIntervals(v) {
			t.Fatal("invalid settings accepted", v)
		}
	}
	if _, _, e = reviewSchedule(now, 6, "good", ints); e == nil {
		t.Fatal("bad level accepted")
	}
	if _, _, e = reviewSchedule(now, 0, "correct", ints); e == nil {
		t.Fatal("bad feedback accepted")
	}
	if _, d, e = reviewSchedule(now, 0, "good", []int{2, 4, 8, 16, 32}); e != nil || d.In(learningZone).Format("2006-01-02") != "2027-01-02" {
		t.Fatal(d, e)
	}
}
