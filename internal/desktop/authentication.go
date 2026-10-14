// SPDX-License-Identifier: GPL-3.0-only
package desktop

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"oshell-desktop/internal/core"
)

type authenticationAnswer struct {
	protection string
	values     []string
	remember   bool
	err        error
}
type authenticationChallenge struct {
	protection          string
	pane                *pane
	id                  string
	ctx                 context.Context
	request             core.AuthenticationPrompt
	answers             []string
	allowSave, remember bool
	open                bool
	done                chan authenticationAnswer
}

func askAuthentication(ctx context.Context, p *pane, request core.AuthenticationPrompt) ([]string, error) {
	result := askAuthenticationWithSaving(ctx, p, request, false)
	return result.values, result.err
}

func askAuthenticationWithSaving(ctx context.Context, p *pane, request core.AuthenticationPrompt, allowSave bool) authenticationAnswer {
	challenge := &authenticationChallenge{allowSave: allowSave, pane: p, id: core.NewID(), ctx: ctx, request: request, answers: make([]string, len(request.Questions)), open: true, done: make(chan authenticationAnswer, 1)}
	if !postPane(p, func(a *App) {
		if ctx.Err() == nil {
			challenge.protection = "local"
			if a.vault != nil {
				challenge.protection = a.vault.CredentialProtection(p.profile.ID)
			}
			a.challenges = append(a.challenges, challenge)
		}
	}) {
		return authenticationAnswer{err: context.Canceled}
	}
	select {
	case result := <-challenge.done:
		return result
	case <-ctx.Done():
		postPane(p, func(a *App) {
			if a.hub != nil {
				for _, w := range a.hub.windows {
					w.removeChallenge(challenge)
				}
			} else {
				a.removeChallenge(challenge)
			}
		})
		return authenticationAnswer{err: ctx.Err()}
	}
}
func (a *App) removeChallenge(challenge *authenticationChallenge) {
	for i, v := range a.challenges {
		if v == challenge {
			a.challenges = append(a.challenges[:i], a.challenges[i+1:]...)
			break
		}
	}
	for i := range challenge.answers {
		challenge.answers[i] = ""
	}
}
func (a *App) authenticationChallengeDialog(c *ui.Context) {
	for len(a.challenges) > 0 && a.challenges[0].ctx.Err() != nil {
		a.removeChallenge(a.challenges[0])
	}
	if len(a.challenges) == 0 {
		return
	}
	q := a.challenges[0]
	finish := func(cancelled bool) {
		result := authenticationAnswer{values: append([]string(nil), q.answers...), remember: q.allowSave && q.remember, protection: q.protection}
		if cancelled {
			result.values = nil
			result.remember = false
			result.err = context.Canceled
		}
		select {
		case q.done <- result:
		default:
		}
		a.removeChallenge(q)
	}
	ui.Modal(c.Key(q.id), &q.open, func() {
		ui.Column(c).Width(500).Gap(12).Children(func() {
			title := q.request.Title
			if title == "" {
				title = "服务器认证"
			}
			ui.Text(c, title).FontSize(18).Bold()
			ui.Text(c, fmt.Sprintf("%s@%s:%d", q.request.Identity.User, q.request.Identity.Host, q.request.Identity.Port))
			if q.request.Name != "" {
				ui.Text(c, q.request.Name)
			}
			if q.request.Instruction != "" {
				ui.Text(c, q.request.Instruction).TextColor(c.Theme().TextMuted)
			}
			ui.Scroll(c).MaxHeight(350).Children(func() {
				for i, prompt := range q.request.Questions {
					ui.Field(c.Key(fmt.Sprint(i)), prompt, func() {
						input := ui.TextInput(c, &q.answers[i]).Label(prompt)
						if !q.request.Echo[i] {
							passwordSelection(input, &q.answers[i])
						}
						if i == 0 {
							input.AutoFocus()
						}
					})
				}
			})
			if q.allowSave {
				ui.Checkbox(c, &q.remember, "密码认证成功后保存登录密码")
				if q.remember {
					passwordProtectionField(c, &q.protection)
				}
			}
			ui.Row(c).Justify(ui.End).Gap(8).Children(func() {
				ui.Button(c, "取消").OnClick(func() { finish(true) })
				ui.PrimaryButton(c, "继续认证").OnClick(func() { finish(false) })
			})
		})
	})
	if !q.open {
		finish(true)
	}
}
