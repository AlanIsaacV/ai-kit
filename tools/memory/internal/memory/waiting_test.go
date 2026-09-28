package memory

import (
	"strings"
	"testing"
)

func TestAnAnsweredIdeaLeavesTheIdeasSection(t *testing.T) {
	kept := entry(t, KindIdea, "A digest of pending items every morning")
	dropped := entry(t, KindIdea, "Colour the blocked group red")
	drop := entry(t, KindDecision, "Colour adds nothing a heading does not")

	userLog := logOf([]*Entry{kept, dropped}, nil)
	repoLog := logOf([]*Entry{drop}, []*Edge{newTestEdge(t, EdgeAnswers, drop.ID, dropped.ID)})
	store := append(append([]Line{}, userLog...), repoLog...)

	out := RenderWaiting([]LogState{
		{Name: "user", Tree: NewTreeIn(userLog, store)},
		{Name: "api", Root: "/src/api", Tree: NewTreeIn(repoLog, store)},
	}, nil)

	var section []string
	in := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, headingIdeas+" ("):
			in = true
		case in && strings.HasPrefix(line, "  "):
			section = append(section, line)
		case in:
			in = false
		}
	}
	body := strings.Join(section, "\n")

	if !strings.Contains(body, kept.ID) || !strings.Contains(body, "user") {
		t.Errorf("the unanswered idea has to be listed with its log's name and id:\n%s", out)
	}
	if strings.Contains(out, dropped.ID) {
		t.Errorf("an idea something answers must not be listed:\n%s", out)
	}
}
