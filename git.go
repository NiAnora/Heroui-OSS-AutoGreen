package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// newAuth 使用 PAT 构造 GitHub HTTPS 认证信息。
func newAuth(token string) *http.BasicAuth {
	return &http.BasicAuth{
		Username: "x-access-token",
		Password: token,
	}
}

// openOrClone 打开本地仓库；若不存在则先 clone。
func openOrClone(repo *Repo, token string) (*git.Repository, error) {
	if _, err := os.Stat(repo.WorkDir); os.IsNotExist(err) {
		log.Printf("cloning %s", repo.RepoURL)
		return git.PlainClone(repo.WorkDir, false, &git.CloneOptions{
			URL:      repo.RepoURL,
			Auth:     newAuth(token),
			Progress: os.Stdout,
		})
	}
	return git.PlainOpen(repo.WorkDir)
}

// syncRepo 拉取远程最新提交，并将本地工作区硬重置到远程 HEAD，
// 丢弃本地任何未推送的提交，保证后续空提交基于最新基线。
func syncRepo(r *git.Repository, repo *Repo, token string) error {
	auth := newAuth(token)
	refSpec := config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", repo.Branch, repo.Branch))

	err := r.Fetch(&git.FetchOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
		Force:    true,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	remoteRef, err := r.Reference(plumbing.NewRemoteReferenceName("origin", repo.Branch), true)
	if err != nil {
		return err
	}

	w, err := r.Worktree()
	if err != nil {
		return err
	}
	return w.Reset(&git.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   git.HardReset,
	})
}

// createEmptyCommit 复用父提交的 tree 构造一次空提交，并返回新提交的 hash。
func createEmptyCommit(r *git.Repository, repo *Repo) (plumbing.Hash, error) {
	head, err := r.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	parent, err := r.CommitObject(head.Hash())
	if err != nil {
		return plumbing.ZeroHash, err
	}

	sig := object.Signature{
		Name:  repo.UserName,
		Email: repo.UserEmail,
		When:  time.Now(),
	}
	commit := &object.Commit{
		Author:       sig,
		Committer:    sig,
		Message:      repo.CommitMsg,
		TreeHash:     parent.TreeHash,
		ParentHashes: []plumbing.Hash{parent.Hash},
	}

	enc := r.Storer.NewEncodedObject()
	if err := commit.Encode(enc); err != nil {
		return plumbing.ZeroHash, err
	}
	hash := enc.Hash()
	if _, err := r.Storer.SetEncodedObject(enc); err != nil {
		return plumbing.ZeroHash, err
	}

	// 更新当前分支引用指向新提交（HEAD 为符号引用时，其目标即分支引用）。
	branchRef := head.Name()
	if head.Type() == plumbing.SymbolicReference {
		branchRef = head.Target()
	}
	if err := r.Storer.SetReference(plumbing.NewHashReference(branchRef, hash)); err != nil {
		return plumbing.ZeroHash, err
	}
	return hash, nil
}

// pushRepo 将本地分支推送到远程。
func pushRepo(r *git.Repository, repo *Repo, token string) error {
	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", repo.Branch, repo.Branch))
	return r.Push(&git.PushOptions{
		Auth:     newAuth(token),
		RefSpecs: []config.RefSpec{refSpec},
	})
}
