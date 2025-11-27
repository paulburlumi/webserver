package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strconv"
	"time"
)

type state struct {
	Rowers  []rower      `json:"rowers"`
	Signals rowerSignals `json:"signals"`
}

type rower struct {
	Name      string
	BirthYear int
	Age       int
	Band      string
}

type rowerSignals struct {
	Name           string `json:"name"`
	BirthYearOrAge string `json:"birthYearOrAge"`
	AverageAge     string `json:"averageAge"`
	AverageBand    string `json:"averageBand"`
	Example        string `json:"example"`
}

type business struct {
	s *store
}

func newBusiness(s *store) *business {
	return &business{s: s}
}

func (b *business) Create(ctx context.Context, key, name, birthYearOrAgeStr string) error {
	s, err := b.getState(ctx, key)
	if err != nil {
		return fmt.Errorf("could not get state: %w", err)
	}

	birthYearOrAge, err := strconv.Atoi(birthYearOrAgeStr)
	if err != nil {
		return fmt.Errorf("invalid birth year or age: %w", err)
	}

	rower, err := newRower(name, birthYearOrAge)
	if err != nil {
		return fmt.Errorf("could not create rower: %w", err)
	}

	slog.Info("Created rower", "rower", rower)
	s.Rowers = append(s.Rowers, rower)

	updateSignals(s)

	if err := b.putState(ctx, key, s); err != nil {
		return fmt.Errorf("could not save state: %w", err)
	}

	return nil
}

func (b *business) Delete(ctx context.Context, key string, index int) error {
	s, err := b.getState(ctx, key)
	if err != nil {
		return fmt.Errorf("could not get state: %w", err)
	}

	if index < 0 || index >= len(s.Rowers) {
		return fmt.Errorf("row not found: %d", index)
	}

	slog.Info("Deleted rower", "rower", s.Rowers[index])
	s.Rowers = slices.Delete(s.Rowers, index, index+1)

	updateSignals(s)

	if err := b.putState(ctx, key, s); err != nil {
		return fmt.Errorf("could not save state: %w", err)
	}

	return nil
}

func (b *business) Watch(ctx context.Context, key string, callback func(*state) error) error {
	s := &state{}
	updateSignals(s)
	if err := callback(s); err != nil {
		return fmt.Errorf("could not execute callback: %w", err)
	}

	callbackWrapper := func(value []byte) error {
		s := &state{}
		if err := json.Unmarshal(value, s); err != nil {
			return fmt.Errorf("could not unmarshal state: %w", err)
		}
		if err := callback(s); err != nil {
			return fmt.Errorf("could not execute callback: %w", err)
		}
		return nil
	}

	if err := b.s.Watch(ctx, key, callbackWrapper); err != nil {
		return fmt.Errorf("could not watch key: %w", err)
	}
	return nil
}

func (b *business) getState(ctx context.Context, key string) (*state, error) {
	s := &state{}
	value, err := b.s.Get(ctx, key)
	if err != nil {
		if !errors.Is(err, ErrKeyNotFound) {
			return nil, fmt.Errorf("could not get state: %w", err)
		}
		return s, nil
	}
	if err := json.Unmarshal(value, s); err != nil {
		return nil, fmt.Errorf("could not unmarshal state: %w", err)
	}
	return s, nil
}

func (b *business) putState(ctx context.Context, key string, s *state) error {
	x, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("could not marshal state: %w", err)
	}
	if err := b.s.Put(ctx, key, x); err != nil {
		return fmt.Errorf("could not save state: %w", err)
	}
	return nil
}

func updateSignals(s *state) {
	averageAge := calculateAverageAge(s.Rowers)
	averageBand := calculateBand(averageAge)

	exampleInputAge := int(minAge + rand.Float64()*(maxAge-minAge))
	exampleInputYear := time.Now().Year() - exampleInputAge

	slog.Info("Updated averages", "averageAge", averageAge, "averageBand", averageBand)
	s.Signals = rowerSignals{
		AverageAge:  fmt.Sprintf("%.1f", averageAge),
		AverageBand: averageBand,
		Example:     fmt.Sprintf("e.g. %d or %d", exampleInputYear, exampleInputAge),
	}
}

var ageBands = []struct {
	Band   string
	MinAge float64
}{
	{"A", 27},
	{"B", 36},
	{"C", 43},
	{"D", 50},
	{"E", 55},
	{"F", 60},
	{"G", 65},
	{"H", 70},
	{"I", 75},
	{"J", 80},
	{"K", 85},
}

var minAge = ageBands[0].MinAge
var maxAge = ageBands[len(ageBands)-1].MinAge

func newRower(name string, birthYearOrAge int) (rower, error) {
	birthYear := birthYearOrAge
	thisYear := time.Now().Year()
	if birthYearOrAge < 200 {
		birthYear = thisYear - birthYearOrAge
	}
	age := thisYear - birthYear
	if age < 1 {
		return rower{}, fmt.Errorf("invalid birth year or age: %d", birthYearOrAge)
	}
	band := calculateBand(float64(age))
	if band == "" {
		return rower{}, fmt.Errorf("%s aged %d is too young for a masters category", name, age)
	}
	return rower{
		Name:      name,
		BirthYear: birthYear,
		Age:       age,
		Band:      band,
	}, nil
}

func calculateAverageAge(rowers []rower) float64 {
	if len(rowers) == 0 {
		return 0.0
	}
	totalAge := 0
	for _, r := range rowers {
		totalAge += r.Age
	}
	return float64(totalAge) / float64(len(rowers))
}

func calculateBand(age float64) string {
	band := ""
	for _, ageBand := range ageBands {
		if ageBand.MinAge > age {
			break
		}
		band = ageBand.Band
	}
	return band
}
