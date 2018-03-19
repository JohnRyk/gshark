/*

Copyright (c) 2018 sec.xiaomi.com

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THEq
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

*/

package githubsearch

import (
	"../../models"

	"github.com/google/go-github/github"

	"encoding/json"
	"time"
	"sync"
	"strings"
	"../../logger"
	"../../vars"
)

var (
	SEARCH_NUM = 25
)

func GenerateSearchCodeTask() (map[int][]models.Rules, error) {
	result := make(map[int][]models.Rules)
	rules, err := models.GetGithubKeywords()
	ruleNum := len(rules)
	batch := ruleNum / SEARCH_NUM

	for i := 0; i < batch; i++ {
		result[i] = rules[SEARCH_NUM*i:SEARCH_NUM*(i+1)]
	}

	if ruleNum%SEARCH_NUM != 0 {
		result[batch] = rules[SEARCH_NUM*batch:ruleNum]
	}
	return result, err
}

func Search(rules []models.Rules) () {
	var wg sync.WaitGroup
	wg.Add(len(rules))
	client, token, err := GetGithubClient()
	if err == nil && token != "" {
		for _, rule := range rules {
			go func(rule models.Rules) {
				defer wg.Done()

			}(rule)

			SaveResult(client.SearchCode(rule.Pattern))
		}
		wg.Wait()
	}
}

func RunSearchTask(mapRules map[int][]models.Rules, err error) () {
	if len(mapRules) == 0 {
		logger.Log.Println("There is no rule enabled!")
	}
	if err == nil {
		for _, rules := range mapRules {
			startTime := time.Now()
			Search(rules)
			usedTime := time.Since(startTime).Seconds()
			if usedTime < 60 {
				time.Sleep(time.Duration(60 - usedTime))
			}
		}
	}
}

func isValidExtension(fullname string) bool{
	for _, extension := range vars.IgnoreFileExtensions {
		if strings.HasSuffix(fullname, extension) {
			return false
		}
	}
	return true
}

func SaveResult(results []*github.CodeSearchResult, err error) () {
	insertCount := 0
	for _, result := range results {
		if err == nil && len(result.CodeResults) > 0 {
			for _, resultItem := range result.CodeResults {
				ret, err := json.Marshal(resultItem)
				if err == nil {
					var codeResult *models.CodeResult
					err = json.Unmarshal(ret, &codeResult)
					fullName := codeResult.Repository.GetFullName()
					repoUrl := codeResult.Repository.GetHTMLURL()
					codeResult.RepoName = fullName

					inputInfo := models.NewInputInfo("repo", repoUrl, fullName)
					has, err := inputInfo.Exist(repoUrl)
					if !isValidExtension(fullName) {
						codeResult.Status = 2
					}
					if err == nil && !has {
						inputInfo.Insert()
					}
					exist, err := codeResult.Exist()
					if err == nil && !exist {
						insertCount++
						logger.Log.Infoln(codeResult.Insert())
					}
				}
			}
		}
		logger.Log.Infof("Has inserted %d results into code_result", insertCount)
	}
}

func ScheduleTasks(duration time.Duration) {
	for {
		RunSearchTask(GenerateSearchCodeTask())

		// insert repos from inputInfo
		InsertAllRepos()

		logger.Log.Infof("Complete the scan of Github, start to sleep %v seconds", duration*time.Second)
		time.Sleep(duration * time.Second)
	}
}