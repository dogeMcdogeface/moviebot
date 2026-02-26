package storage

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type fieldFormatter func(Movie) string

type MovieColumn struct {
	Header string
	Width  int
	Format fieldFormatter
}

type TableFormat struct {
	Columns         []MovieColumn
	SortBy          sortMethod
	SeparateWatched bool
}

type sortMethod int

const (
	SortByVotes sortMethod = iota
	SortByDateAdded
)




func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func FormatTitle(m Movie) string {
	if m.Title == "" {
		return "???"
	}
	return fmt.Sprintf("%s", m.Title)
}

func FormatYear(m Movie) string {
	if m.Year <= 0 || m.Year > time.Now().Year()+1 {
		return "???"
	}
	return fmt.Sprintf("%4d", m.Year)
}

func FormatVotes(m Movie) string {
	return fmt.Sprintf("%5d", len(m.Votes))
}

func FormatWatched(m Movie) string {
	return fmt.Sprintf("%4d", len(m.Watched))
}

func FormatAdded(m Movie) string {
	return timeAgo(m.AddedAt)
}

func timeAgo(addedAt time.Time) string {
	now := time.Now()
	diff := now.Sub(addedAt)

	switch {
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	case diff < 30*24*time.Hour: // Rough estimate for a month
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%dM ago", int(diff.Hours()/(24*30))) // Assume 30 days in a month
	default:
		return fmt.Sprintf("%dY ago", int(diff.Hours()/(24*365))) // Assume 365 days in a year
	}
}

// Helper functions for sorting
func sortMoviesByVotes(movies []Movie) {
	sort.Slice(movies, func(i, j int) bool {
		return len(movies[i].Votes) > len(movies[j].Votes)
	})
}

func sortMoviesByDateAdded(movies []Movie) {
	sort.Slice(movies, func(i, j int) bool {
		return movies[i].AddedAt.Before(movies[j].AddedAt)
	})
}

func BuildListMessage(movies []Movie, format TableFormat) string {
	// Extract the fields from the format struct
	columns := format.Columns
	sortBy := format.SortBy
	separateWatched := format.SeparateWatched

	if len(movies) == 0 {
		return "No movies yet"
	}

	// Sort movies based on the selected method
	switch sortBy {
	case SortByVotes:
		sortMoviesByVotes(movies)
	case SortByDateAdded:
		sortMoviesByDateAdded(movies)
	}

	var sb strings.Builder

	// Print header with "|" separator between columns
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(" | ") // Add pipe separator
		}
		sb.WriteString(fmt.Sprintf("%-*s", col.Width, col.Header))
	}
	sb.WriteString("\n")

	for i, col := range columns {
		if i > 0 {
			sb.WriteString("-+-") // Add pipe separator
		}
		sb.WriteString(strings.Repeat("-", col.Width))
	}
	sb.WriteString("\n")

	// Separate watched and unwatched movies if needed
	var unwatched, watched []Movie
	if separateWatched {
		for _, m := range movies {
			if len(m.Watched) > 0 && len(m.Watched) >= len(m.Votes) {
				watched = append(watched, m)
			} else {
				unwatched = append(unwatched, m)
			}
		}
	} else {
		watched = movies
	}

	// Function to write a movie's information to the string builder
	writeMovie := func(m Movie) {
		for i, col := range columns {
			if i > 0 {
				sb.WriteString(" | ") // Add pipe separator
			}
			sb.WriteString(truncate(fmt.Sprintf("%-*s", col.Width, col.Format(m)),col.Width))
		}
		sb.WriteString("\n")
	}

	// Write unwatched movies
	//if separateWatched {
	//	sb.WriteString("Unwatched:\n")
	//}
	for _, m := range unwatched {
		writeMovie(m)
	}

if separateWatched && len(watched) > 0 {
	sb.WriteString("\n")
	// Compute table width
	width := 0
	for _, col := range columns {
		width += col.Width
	}
	width += (len(columns) - 1) * 3 // account for " | "
	text := "Watched"
	padding := width - len(text)
	sb.WriteString(strings.Repeat("-", padding/2) + text + strings.Repeat("-", padding-padding/2) + "\n")

	for _, m := range watched {
		writeMovie(m)
	}
}

	return sb.String()
}

