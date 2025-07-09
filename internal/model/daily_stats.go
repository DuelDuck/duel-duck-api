package model

import "fmt"

type DailyStats struct {
	TotalUsers    int `json:"total_users"`
	TodayNewUsers int `json:"new_daily_users"`

	TotalUserWin int `json:"total_user_win"`
	TodayUserWin int `json:"total_user_win_today"`

	TotalCreatedDuels int `json:"created_duels"`
	TodayCreatedDuels int `json:"created_duels_today"`

	TotalCompleteDuels int `json:"complete_duels"`
	TodayCompleteDuels int `json:"complete_duels_today"`
}

func (s DailyStats) TgMessageFmt() string {
	msg := fmt.Sprintf(
		`📊 *Daily Stats* 📊

		*Total Users:* %d
		*Today New Users:* %d
			
		*Total Users Win:* %d
		*Today Users Win:* %d

		*Total Created Duels:* %d
		*Today Created Duels:* %d

		*Total Completed Duels:* %d
		*Today Completed Duels:* %d`,
		s.TotalUsers,
		s.TodayNewUsers,

		s.TotalUserWin,
		s.TodayUserWin,

		s.TotalCreatedDuels,
		s.TodayCreatedDuels,

		s.TotalCompleteDuels,
		s.TodayCompleteDuels,
	)

	return msg
}
