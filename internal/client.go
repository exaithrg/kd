package internal

/*

功能：

- 查询
- 更新
*/

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Karmenzind/kd/internal/cache"
	"github.com/Karmenzind/kd/internal/core"
	"github.com/Karmenzind/kd/internal/daemon"
	"github.com/Karmenzind/kd/internal/model"
	q "github.com/Karmenzind/kd/internal/query"
	d "github.com/Karmenzind/kd/pkg/decorate"
	"go.uber.org/zap"
)

func ensureDaemon(running chan bool) {
	if !daemon.ServerIsRunning() {
		err := daemon.StartDaemonProcess()
		if err != nil {
            d.EchoRun("未找到守护进程，正在启动...")
			d.EchoFatal(err.Error())
		}
		running <- true
	}
    running <- true
}

func Query(query string, noCache bool) (r *model.Result, err error) {
	query = strings.ToLower(strings.Trim(query, " "))

	r = &model.Result{Query: query}
	r.History = make(chan int, 1)

	daemonRunning := make(chan bool)
	go ensureDaemon(daemonRunning)

	core.WG.Add(1)
	go cache.CounterIncr(query, r.History)

	// any valid char
	if m, _ := regexp.MatchString("^[a-zA-Z0-9\u4e00-\u9fa5]", query); !m {
		r.Found = false
		r.Prompt = "请输入有效查询字符或参数"
		return
	}

	line, err := cache.CheckNotFound(r.Query)
	if err != nil {
		zap.S().Warnf("[cache] check not found error: %s", err)
	} else if line > 0 {
		r.Found = false
		zap.S().Debugf("`%s` is in not-found-list", r.Query)
		return
	}
    r.Initialize()

	if !noCache {
		cacheErr := q.FetchCached(r)
		if cacheErr != nil {
			zap.S().Warnf("[cache] Query error: %s", cacheErr)
		}
		if r.Found {
			return
		}
		_ = err
	}

	if <-daemonRunning {
		err = QueryDaemon(r)
	} else {
		d.EchoFatal("守护进程未启动，请手动执行`kd --daemon`")
	}

	// FIXME move to server
	// if !r.Found {
	// 	err = q.FetchOnline(r)
	// 	// 判断时间
	// 	cache.UpdateQueryCache(r)
	// }
	return r, err
}

func QueryDaemon(r *model.Result) error {
	addr := fmt.Sprintf("localhost:%d", SERVER_PORT)
	err := q.QueryDaemon(addr, r)
	return err
}