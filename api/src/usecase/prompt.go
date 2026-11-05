package usecase

import (
	"fmt"
	"strings"

	"github.com/Satoru314/Kobo-13900K/api/src/domain"
)

// prompts builds the text handed to the AI. The wording is in docs/design.md.
type prompts struct {
	maxRounds int
}

func (p prompts) develop(t domain.Task, pj domain.Project, workDir string) string {
	return fmt.Sprintf(`あなたは Kobo から開発タスクを受け取った開発担当です。
作業ディレクトリ %[1]s は %[2]s の %[3]s から作った git worktree で、ブランチ %[4]s をチェックアウトしています。

# タスク
%[5]s

%[6]s

# 進め方
- このディレクトリの中だけで作業する。リポジトリの AGENTS.md / CLAUDE.md があれば従う
- ブランチ名は %[4]s のまま使う
- 質問はできない。迷ったら妥当な方を選んで進め、選んだ内容を PR に書く
- テストやビルドの方法があれば、PR を出す前に通す
- 画面の動作確認は Playwright のブラウザで行う
- 終わったら push し、%[3]s 向けの PR を gh で作る

# PR の本文
- 何をしたか
- 自分で判断して決めたこと（仕様が書かれていなかった所、複数の選択肢から選んだ所）と、その理由。なければ「なし」
- 確認したこと（実行したテストなど）

# 終わり方
- PR を作ったら kobo-agent の complete_development ツールを呼ぶ（PR の URL と要約）
- 進められない場合（仕様に無理がある、外部の問題でテストや CI が通らない等）は、PR を作らずに kobo-agent の report_blocked ツールを呼び、理由を書く
- どちらかを呼んだら、それ以上は何もしない
`, workDir, pj.FullName(), pj.DefaultBranch, t.Branch(), t.Title, t.Description)
}

func (p prompts) fix(pr domain.PullRequest, workDir string, round domain.ReviewRound) string {
	kind := "レビューの指摘"
	if round.Result == domain.ReviewCIFailed {
		kind = "CI の失敗"
	}
	return fmt.Sprintf(`PR %[1]s に次の%[2]sがありました。対応してください。
作業ディレクトリ %[3]s は PR のブランチ %[4]s から作り直した worktree です。

# %[2]s
%[5]s

# 進め方
- 指摘ごとに直す。直さないと判断したものは理由を書く
- 同じブランチ %[4]s に push する（新しい PR は作らない）
- PR に、何をどう直したか、自分で判断して決めたことをコメントで書く
- 画面の動作確認は Playwright のブラウザで行う

# 終わり方
- push したら kobo-agent の complete_development ツールを呼ぶ（PR の URL と要約）
- 進められない場合は kobo-agent の report_blocked ツールを呼ぶ
- どちらかを呼んだら、それ以上は何もしない
`, pr.URL, kind, workDir, pr.Branch, round.Comments)
}

func (p prompts) review(t domain.Task, pr domain.PullRequest, workDir string, roundNo int) string {
	again := ""
	if roundNo > 1 {
		again = "- 前回の指摘が直ったかを確認する\n"
	}
	return fmt.Sprintf(`あなたは Kobo から PR のレビューを頼まれたレビュー担当です。
PR: %[1]s（%[2]d 周目 / 最大 %[3]d 周）
作業ディレクトリ %[4]s には PR のブランチがチェックアウトされています。

# 元のタスク
%[5]s

%[6]s

# 見ること
- タスクの内容を満たしているか
- バグ、壊れるケース、セキュリティの問題
- PR に書かれた「自分で判断して決めたこと」が妥当か
- 好みの問題や細かい書き方は指摘しない。直さないとマージできないものだけを指摘する
- 画面の動作確認は Playwright のブラウザで行う
%[7]s
# 書き方
- 指摘は gh で PR のレビューコメントとして書く（該当する行に付けられるものは行に付ける）
- コードは変更しない。commit も push もしない

# 終わり方
- 指摘がなければ kobo-agent の complete_review ツールを result=approved で呼ぶ
- 指摘があれば kobo-agent の complete_review ツールを result=changes_requested で呼び、指摘の一覧を comments に渡す
- 呼んだら、それ以上は何もしない
`, pr.URL, roundNo, p.maxRounds, workDir, t.Title, strings.TrimSpace(t.Description), again)
}
