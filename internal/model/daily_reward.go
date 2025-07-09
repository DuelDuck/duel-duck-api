package model

import (
	"time"
)

const (
	Day                       = 24 * time.Hour
	DailyRewardWeeklyCycle    = 7
)

type DailyReward struct {
	DayStreak int  `json:"day_streak"`
	Claimed   bool `json:"claimed"`
}

func hasClaimedRewardToday(lastCompletedStreak time.Time) bool {
	todayRewardUpdateTime := time.Now().UTC().Truncate(Day)
	return lastCompletedStreak.UTC().Truncate(Day).Equal(todayRewardUpdateTime)
}

func hasClaimedRewardYesterday(lastCompletedStreak time.Time) bool {
	prevRewardUpdateTime := time.Now().UTC().Truncate(Day).Add(-Day)
	return lastCompletedStreak.UTC().After(prevRewardUpdateTime)
}

func (r DailyReward) RewardDayByStreak() int {
	rewardMultiplier := r.DayStreak % DailyRewardWeeklyCycle
	if rewardMultiplier == 0 {
		rewardMultiplier = 7
	}

	return rewardMultiplier
}

func CurrentDailyReward(user *User) *DailyReward {
	reward := &DailyReward{
		DayStreak: user.DailyRewardStreak,
		Claimed:   hasClaimedRewardToday(user.LastCompletedStreak),
	}

	if reward.Claimed {
		return reward
	}

	if hasClaimedRewardYesterday(user.LastCompletedStreak) {
		reward.DayStreak += 1
	} else {
		reward.DayStreak = 1
	}

	return reward
}

func CycleDayStreak(user *User) *DailyReward {
	dailyReward := CurrentDailyReward(user)
	dailyReward.DayStreak = dailyReward.RewardDayByStreak()

	return dailyReward
}
