package usecase

import (
	"strings"
	"testing"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

func TestDevelopAndFixPromptsTellWhereScreenshotsGo(t *testing.T) {
	p := prompts{maxRounds: 2, shotsRepo: "acme/shots"}
	task := domain.Task{ID: 7, Title: "title"}
	pj := domain.Project{Owner: "acme", Name: "app", DefaultBranch: "main"}
	pr := domain.PullRequest{URL: "https://example.com/pr/1", Branch: "kobo/task-7"}

	const url = "https://github.com/acme/shots/blob/main/app/task-7-<連番>.png?raw=true"
	for name, c := range map[string]struct{ prompt, postTo string }{
		"develop": {p.develop(task, pj, "/wt"), "PR の本文に、"},
		"fix":     {p.fix(task, pj, pr, "/wt", domain.ReviewRound{Comments: "x"}), "PR のコメントに、"},
	} {
		for _, want := range []string{"acme/shots の main ブランチの app/task-7-<連番>.png", url, c.postTo, "画面に関わらない変更なら撮らない"} {
			if !strings.Contains(c.prompt, want) {
				t.Errorf("%s prompt lacks %q:\n%s", name, want, c.prompt)
			}
		}
		if strings.Contains(c.prompt, "%!") {
			t.Errorf("%s prompt has a format error:\n%s", name, c.prompt)
		}
	}
}

func TestDevelopPromptStopsOnLargeOpenQuestions(t *testing.T) {
	p := prompts{maxRounds: 2, shotsRepo: "acme/shots"}
	task := domain.Task{ID: 7, Title: "title"}
	pj := domain.Project{Owner: "acme", Name: "app", DefaultBranch: "main"}
	got := p.develop(task, pj, "/wt")

	for _, want := range []string{
		"質問はできない。細かい未定は妥当な方を選んで進め、選んだ内容を PR に書く",
		"ただし、どちらを選ぶかで作るものが大きく変わる未定（画面の構成、データの持ち方、外から見える振る舞いなど）が要件にあれば、作業を始めずに report_blocked を呼び、何が決まっていないかと、考えられる選択肢を書く",
		"進められない場合（要件に大きな未定がある、仕様に無理がある、外部の問題でテストや CI が通らない等）",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("develop prompt lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "迷ったら妥当な方を選んで進め") {
		t.Errorf("develop prompt still tells the AI to proceed when unsure:\n%s", got)
	}
}

func TestReviewPromptAsksForMissingScreenshots(t *testing.T) {
	p := prompts{maxRounds: 2, shotsRepo: "acme/shots"}
	got := p.review(domain.Task{ID: 7}, domain.PullRequest{}, "/wt", 2)
	if !strings.Contains(got, "スクリーンショットを求める指摘をする") {
		t.Errorf("review prompt does not ask for screenshots:\n%s", got)
	}
	if strings.Contains(got, "%!") {
		t.Errorf("review prompt has a format error:\n%s", got)
	}
}
