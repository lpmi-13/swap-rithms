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
	SignupAt    time.Time
	Score       int
	Region      string
	Bio         string
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
		region := generatedRegion(i)
		p := Profile{
			ID:          i + 1,
			Name:        fmt.Sprintf("User %06d", i+1),
			Email:       fmt.Sprintf("user%06d@example.test", i+1),
			LastUpdated: generatedLastUpdated(now, i, count),
			SignupAt:    generatedSignupAt(now, i, count),
			Score:       generatedScore(i),
			Region:      region,
			Bio:         generatedBio(i, region),
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

func generatedSignupAt(now time.Time, index int, count int) time.Time {
	window := 180 * 24 * time.Hour
	step := window / time.Duration(max(count, 1))
	age := time.Duration(index)*step + time.Duration((index*53)%997)*time.Millisecond
	return now.Add(-window + age)
}

func generatedScore(index int) int {
	return ((index + 1) * 7919) % 1001
}

func generatedRegion(index int) string {
	regions := [...]string{"na", "eu", "apac", "latam", "mea"}
	return regions[index%len(regions)]
}

func generatedBio(index int, region string) string {
	topics := [...]string{"platform", "analytics", "billing", "support", "security", "growth", "mobile", "search"}
	roles := [...]string{"admin", "operator", "creator", "reviewer", "developer"}
	return fmt.Sprintf("%s %s user focused on %s workflows in %s", roles[index%len(roles)], region, topics[(index*7)%len(topics)], region)
}

func generatedPayload(seed int) [profilePayloadSize]byte {
	var payload [profilePayloadSize]byte
	for i := range payload {
		payload[i] = byte('a' + ((seed + i) % 26))
	}
	return payload
}
