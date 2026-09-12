package main

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sort"
	"time"
)

// 调度模式：weekly 每周真随机 N 天；daily 每天真随机决定是否提交。
const (
	ScheduleWeekly = "weekly"
	ScheduleDaily  = "daily"
)

// dateLayout 用于按天做去重的日期格式。
const dateLayout = "2006-01-02"

// ScheduleConfig 是全局提交计划配置，所有启用的仓库共用同一套。
// 提交时刻在 [HourFrom,HourTo] : [MinuteFrom,MinuteTo] : [SecondFrom,SecondTo]
// 三个闭区间内分别真随机取值。
type ScheduleConfig struct {
	Mode       string `json:"mode"` // weekly | daily
	WeekDays   int    `json:"weekDays"`
	HourFrom   int    `json:"hourFrom"`
	HourTo     int    `json:"hourTo"`
	MinuteFrom int    `json:"minuteFrom"`
	MinuteTo   int    `json:"minuteTo"`
	SecondFrom int    `json:"secondFrom"`
	SecondTo   int    `json:"secondTo"`
}

// defaultScheduleConfig 返回默认计划：每周随机 3 天，时间全天 24 小时全随机。
func defaultScheduleConfig() ScheduleConfig {
	return ScheduleConfig{
		Mode:       ScheduleWeekly,
		WeekDays:   3,
		HourFrom:   0,
		HourTo:     23,
		MinuteFrom: 0,
		MinuteTo:   59,
		SecondFrom: 0,
		SecondTo:   59,
	}
}

// normalizeScheduleConfig 校正配置，保证档位合法、范围有序且不越界。
func normalizeScheduleConfig(c *ScheduleConfig) {
	if c.Mode != ScheduleDaily {
		c.Mode = ScheduleWeekly
	}
	if c.WeekDays < 1 {
		c.WeekDays = 1
	}
	if c.WeekDays > 7 {
		c.WeekDays = 7
	}
	c.HourFrom, c.HourTo = clampRange(c.HourFrom, c.HourTo, 0, 23)
	c.MinuteFrom, c.MinuteTo = clampRange(c.MinuteFrom, c.MinuteTo, 0, 59)
	c.SecondFrom, c.SecondTo = clampRange(c.SecondFrom, c.SecondTo, 0, 59)
}

// clampRange 将区间裁剪到 [min,max] 内，并在起止颠倒时自动交换。
func clampRange(from, to, min, max int) (int, int) {
	if from < min {
		from = min
	}
	if from > max {
		from = max
	}
	if to < min {
		to = min
	}
	if to > max {
		to = max
	}
	if from > to {
		from, to = to, from
	}
	return from, to
}

// randInt 用真随机源生成 [0,n) 内均匀分布的整数。
// crypto/rand.Int 内部采用拒绝采样，不存在取模偏差。
func randInt(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("randInt: n 必须为正数")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// randInRange 在闭区间 [from,to] 内用真随机源均匀取一个整数。
func randInRange(from, to int) (int, error) {
	if from > to {
		from, to = to, from
	}
	n, err := randInt(to - from + 1)
	if err != nil {
		return 0, err
	}
	return from + n, nil
}

// randTimeInDay 在指定日期的给定时间范围内随机取一个时刻：
// 小时、分钟、秒分别在各自区间内真随机，因此区间内任意一秒概率均等。
func randTimeInDay(day time.Time, cfg ScheduleConfig) (time.Time, error) {
	h, err := randInRange(cfg.HourFrom, cfg.HourTo)
	if err != nil {
		return time.Time{}, err
	}
	m, err := randInRange(cfg.MinuteFrom, cfg.MinuteTo)
	if err != nil {
		return time.Time{}, err
	}
	s, err := randInRange(cfg.SecondFrom, cfg.SecondTo)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, s, 0, day.Location()), nil
}

// startOfWeek 返回 t 所在周的周一 00:00（以周一作为一周的起点）。
func startOfWeek(t time.Time) time.Time {
	offset := (int(t.Weekday()) + 6) % 7 // 周一 -> 0，周日 -> 6
	d := t.AddDate(0, 0, -offset)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location())
}

// pickDayOffsets 用真随机洗牌（Fisher-Yates），从一周 7 天
// （0=周一 … 6=周日）中选出 n 个不重复的天，并按先后顺序返回。
func pickDayOffsets(n int) ([]int, error) {
	all := []int{0, 1, 2, 3, 4, 5, 6}
	for i := len(all) - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return nil, err
		}
		all[i], all[j] = all[j], all[i]
	}
	out := append([]int(nil), all[:n]...)
	sort.Ints(out)
	return out, nil
}

// schedulePlan 是一次调度计算的结果：下一次提交时刻，
// 以及在周模式中本周选中的天与需持久化的去重日期。
type schedulePlan struct {
	Next     time.Time
	PlanWeek string // weekly：计划所属周的周一日期
	PlanDays []int  // weekly：本周选中的天偏移（0=周一）
	PlanDate string // 已规划提交的日期，用于保证每天最多提交一次
}

// nextCommitPlan 依据全局计划配置与仓库已有的去重状态，计算下一次提交计划。
func nextCommitPlan(repo *Repo, cfg ScheduleConfig, now time.Time) (schedulePlan, error) {
	if cfg.Mode == ScheduleDaily {
		return nextDailyPlan(repo, cfg, now)
	}
	return nextWeeklyPlan(repo, cfg, now)
}

// nextDailyPlan 实现「每天随机 0 或 1」：以 1/2 的真随机概率决定某天是否提交
// （0 为不提交、1 为提交），确定提交的当天再在配置的时间范围内随机一个时刻。
// 当天一旦已规划过（PlanDate 命中），则从次日开始重新判定，保证每天最多提交一次。
func nextDailyPlan(repo *Repo, cfg ScheduleConfig, now time.Time) (schedulePlan, error) {
	startOffset := 0
	if repo.PlanDate == now.Format(dateLayout) {
		startOffset = 1
	}

	for offset := startOffset; offset < 64; offset++ {
		day := now.AddDate(0, 0, offset)
		hit, err := randInt(2)
		if err != nil {
			return schedulePlan{}, err
		}
		if hit == 0 {
			continue // 该天不提交
		}
		t, err := randTimeInDay(day, cfg)
		if err != nil {
			return schedulePlan{}, err
		}
		if offset == 0 && !t.After(now) {
			continue // 今日的随机时刻已过，改判次日
		}
		return schedulePlan{Next: t, PlanDate: day.Format(dateLayout)}, nil
	}
	return schedulePlan{}, errors.New("无法计算下一次提交时间")
}

// nextWeeklyPlan 实现「每周随机 N 天」：每周用真随机洗牌选出 N 个互不相同的星期几，
// 每个被选中的日子再在配置的时间范围内随机一个时刻，因此每周恰好提交 N 次、每天至多一次。
func nextWeeklyPlan(repo *Repo, cfg ScheduleConfig, now time.Time) (schedulePlan, error) {
	n := cfg.WeekDays
	if n < 1 {
		n = 1
	}
	if n > 7 {
		n = 7
	}

	currentWeek := startOfWeek(now)
	weekStart := currentWeek
	days := repo.PlanDays

	// 最多向后推算 5 周，足以跨过当前周剩余天数不足的情况。
	for i := 0; i < 5; i++ {
		weekKey := weekStart.Format(dateLayout)
		reuse := weekStart.Equal(currentWeek) && repo.PlanWeek == weekKey && len(days) == n
		if !reuse {
			picked, err := pickDayOffsets(n)
			if err != nil {
				return schedulePlan{}, err
			}
			days = picked
		}

		for _, offset := range days {
			day := weekStart.AddDate(0, 0, offset)
			key := day.Format(dateLayout)
			if key <= repo.PlanDate {
				continue // 该天已规划/已提交过
			}
			t, err := randTimeInDay(day, cfg)
			if err != nil {
				return schedulePlan{}, err
			}
			if t.After(now) {
				return schedulePlan{
					Next:     t,
					PlanWeek: weekKey,
					PlanDays: days,
					PlanDate: key,
				}, nil
			}
		}
		weekStart = weekStart.AddDate(0, 0, 7)
	}
	return schedulePlan{}, errors.New("无法计算下一次提交时间")
}
