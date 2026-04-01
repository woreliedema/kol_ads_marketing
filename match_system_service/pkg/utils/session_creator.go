package utils

import "fmt"

// GenerateSessionID 生成 SessionID
func GenerateSessionID(userA, userB uint64) string {
	if userA < userB {
		return fmt.Sprintf("%d_%d", userA, userB)
	}
	return fmt.Sprintf("%d_%d", userB, userA)
}
