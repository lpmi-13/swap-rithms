package main

import (
	"fmt"
	"sort"
	"time"
)

const profilePayloadSize = 384

type Profile struct {
	ID          int
	Name        string
	Email       string
	LastUpdated time.Time
	Payload     [profilePayloadSize]byte
}

type Dataset struct {
	Profiles       []Profile
	ProfilesSorted []Profile
	ProfileMap     map[int]Profile
	GeneratedAt    time.Time
}

func NewDataset(count int) *Dataset {
	now := time.Now().UTC().Truncate(time.Second)
	profiles := make([]Profile, count)
	profileMap := make(map[int]Profile, count)

	for i := 0; i < count; i++ {
		p := Profile{
			ID:          i + 1,
			Name:        fmt.Sprintf("User %06d", i+1),
			Email:       fmt.Sprintf("user%06d@example.test", i+1),
			LastUpdated: generatedLastUpdated(now, i, count),
			Payload:     generatedPayload(i),
		}
		profiles[i] = p
		profileMap[p.ID] = p
	}

	sortedProfiles := append([]Profile(nil), profiles...)
	sort.Slice(sortedProfiles, func(i, j int) bool {
		if sortedProfiles[i].LastUpdated.Equal(sortedProfiles[j].LastUpdated) {
			return sortedProfiles[i].ID < sortedProfiles[j].ID
		}
		return sortedProfiles[i].LastUpdated.Before(sortedProfiles[j].LastUpdated)
	})

	return &Dataset{
		Profiles:       profiles,
		ProfilesSorted: sortedProfiles,
		ProfileMap:     profileMap,
		GeneratedAt:    now,
	}
}

func generatedLastUpdated(now time.Time, index int, count int) time.Time {
	// Spread profiles across the last 24 hours, then add deterministic jitter.
	window := 24 * time.Hour
	step := window / time.Duration(max(count, 1))
	age := time.Duration(index)*step + time.Duration((index*37)%997)*time.Millisecond
	return now.Add(-window + age)
}

func generatedPayload(seed int) [profilePayloadSize]byte {
	var payload [profilePayloadSize]byte
	for i := range payload {
		payload[i] = byte('a' + ((seed + i) % 26))
	}
	return payload
}
