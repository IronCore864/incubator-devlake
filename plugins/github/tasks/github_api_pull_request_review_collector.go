package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/merico-dev/lake/plugins/helper"
	"net/http"
	"net/url"
	"reflect"

	"github.com/merico-dev/lake/plugins/core"
	"github.com/merico-dev/lake/plugins/github/models"
)

const RAW_PULL_REQUEST_REVIEW_TABLE = "github_api_pull_request_reviews"

// this struct should be moved to `gitub_api_common.go`

var _ core.SubTaskEntryPoint = CollectApiPullRequestReviews

func CollectApiPullRequestReviews(taskCtx core.SubTaskContext) error {
	db := taskCtx.GetDb()
	data := taskCtx.GetData().(*GithubTaskData)

	incremental := false

	cursor, err := db.Model(&models.GithubPullRequest{}).
		Joins("left join github_repos on github_repos.github_id = github_pull_requests.repo_id").
		Where("github_repos.`name` = ? and github_repos.owner_login = ?", data.Options.Repo, data.Options.Owner).
		Select("number, github_pull_requests.github_id").
		Rows()
	if err != nil {
		return err
	}
	iterator, err := helper.NewCursorIterator(db, cursor, reflect.TypeOf(SimplePr{}))
	if err != nil {
		return err
	}
	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			/*
				This struct will be JSONEncoded and stored into database along with raw data itself, to identity minimal
				set of data to be process, for example, we process JiraIssues by Board
			*/
			Params: GithubApiParams{
				Owner: data.Options.Owner,
				Repo:  data.Options.Repo,
			},

			/*
				Table store raw data
			*/
			Table: RAW_PULL_REQUEST_REVIEW_TABLE,
		},
		ApiClient:   data.ApiClient,
		PageSize:    100,
		Incremental: incremental,
		Input:       iterator,

		UrlTemplate: "repos/{{ .Params.Owner }}/{{ .Params.Repo }}/pulls/{{ .Input.Number }}/reviews",

		Query: func(pager *helper.Pager) (url.Values, error) {
			query := url.Values{}
			query.Set("state", "all")
			query.Set("page", fmt.Sprintf("%v", pager.Page))
			query.Set("direction", "asc")
			query.Set("per_page", fmt.Sprintf("%v", pager.Size))

			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, error) {
			var items []json.RawMessage
			err := core.UnmarshalResponse(res, &items)
			if err != nil {
				return nil, err
			}
			return items, nil
		},
	})

	if err != nil {
		return err
	}
	return collector.Execute()
}