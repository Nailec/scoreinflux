package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"scoreinflux/db"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "net/http/pprof"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//const ctxDeadline = 30 * time.Second
const urlPrefix = "https://fr.soccerway.com"
const mdb = "xstats"

type Context struct {
	Browser      *rod.Browser
	MongoClient  *db.MongoDBClient
	Competition  string
	Competitions []string
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:8095", nil))
	}()

	browser := rod.New().MustConnect()
	defer browser.MustClose()
	//	ctx, cancel := context.WithTimeout(context.Background(), ctxDeadline)
	//	defer cancel()
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println("mongo error", err)
		return
	}

	c := &Context{
		Browser: browser,
		MongoClient: &db.MongoDBClient{
			Client:   client,
			Database: mdb,
		},
		Competitions: []string{
			//"/international/europe/european-championship-qualification/2020/s12204/final-stages/",
			//"/international/europe/european-championship-qualification/2020/qualifying-round/r46025/",
			//"/international/europe/uefa-nations-league/20182019/s15372/final-stages/",
			//"/international/europe/uefa-nations-league/20182019/league-a/r45718/",
			//"/international/europe/uefa-nations-league/20182019/league-b/r45719/",
			//"/international/europe/uefa-nations-league/20182019/league-c/r45720/",
			//"/international/europe/uefa-nations-league/20202021/league-a/r54499/",
			//"/international/europe/uefa-nations-league/20202021/league-b/r54500/",
			//"/international/europe/uefa-nations-league/20202021/league-c/r54501/",
			"/international/europe/uefa-nations-league/20182019/s15372/final-stages/",
		},
	}

	for _, competition := range c.Competitions {
		c.Competition = competition
		err = client.Database(mdb).Collection("competitions").
			FindOne(context.TODO(), bson.M{"_id": c.Competition}).
			Err()
		if err == nil {
			fmt.Println("already parsed competition " + competition)
			continue
		}
		if err != mongo.ErrNoDocuments {
			fmt.Println("cannot connect to mongodb", err)
			return
		}

		fail := true
		var urls []string
		for fail {
			urls = c.getCompetitionGames(urlPrefix + c.Competition)
			checkDuplicates := map[string]interface{}{}
			fmt.Println(c.Competition)
			fmt.Println(len(urls))

			for _, url := range urls {
				if _, ok := checkDuplicates[url]; ok {
					fmt.Println("fail")
					fail = true
					break
				} else {
					fail = false
					checkDuplicates[url] = nil
				}
			}
		}

		pool := rod.NewPagePool(6)
		create := func() *rod.Page {
			return browser.MustIncognito().MustPage()
		}

		var wg sync.WaitGroup
		for i := range urls {
			if b, _ := c.MongoClient.GameExists(urls[i]); b {
				fmt.Println("already parsed game " + urls[i])
				continue
			}

			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				page := pool.Get(create)
				defer pool.Put(page)

				page.MustNavigate(urls[i]).MustWaitLoad()
				err2 := c.updateWithGame(page, urls[i])
				if err2 != nil {
					fmt.Println(err2)
				}
			}(i)
		}

		wg.Wait()

		err = c.MongoClient.InsertCompetition(c.Competition)
		if err != nil {
			panic(err)
		}
	}
}

func (c *Context) getCompetitionGames(url string) []string {
	page := c.Browser.MustPage(url)
	defer page.Close()

	allMatches := []string{}
	content := page.MustElement(".content-column")
	if content.MustHas(".block_competition_matches_full") {
		fmt.Println("Assuming cup competition")
		blocks := content.MustElements(".block_competition_matches_full")
		for _, block := range blocks {
			matches := block.MustElement("tbody").MustElements(".match")
			for _, match := range matches {
				m := match.MustElement(".score-time").MustElement("a").MustAttribute("href")
				if m == nil {
					continue
				}
				allMatches = append(allMatches, urlPrefix+*m)
			}
		}

		return allMatches
	}

	block := content.MustElement(".block_competition_matches_summary")

	if block.MustHas("select") {
		fmt.Println("Assuming league with game weeks")
		selector := block.MustElement("select")
		for _, val := range selector.MustElements("option") {
			selector.MustSelect(val.MustText())
			time.Sleep(1000 * time.Millisecond)
			block = content.MustElement(".block_competition_matches_summary")
			matches := block.MustElement("tbody").MustElements(".match")
			for _, match := range matches {
				m := match.MustElement(".score-time").MustElement("a").MustAttribute("href")
				if m == nil {
					continue
				}
				allMatches = append(allMatches, urlPrefix+*m)
			}
		}

		return allMatches
	}

	fmt.Println("Assuming disorganized league")
	matchMap := map[string]interface{}{}
	time.Sleep(1000 * time.Millisecond)
	// Click on agree cookies
	if page.MustHas(".qc-cmp2-summary-buttons") {
		el := page.MustElement(".qc-cmp2-summary-buttons")
		el.MustElements("button")[1].MustClick()
	}
	for block.MustHas(".previous:not(.disabled)") {
		matches := block.MustElement("tbody").MustElements(".match")
		for _, match := range matches {
			m := match.MustElement(".score-time").MustElement("a").MustAttribute("href")
			if m == nil {
				continue
			}
			matchMap[urlPrefix+*m] = nil
		}
		previous := block.MustElement(".previous")
		previous.MustClick()
		time.Sleep(1000 * time.Millisecond)
		block = content.MustElement(".block_competition_matches_summary")
	}

	matches := block.MustElement("tbody").MustElements(".match")
	for _, match := range matches {
		m := match.MustElement(".score-time").MustElement("a").MustAttribute("href")
		if m == nil {
			continue
		}
		matchMap[urlPrefix+*m] = nil
	}

	for url, _ := range matchMap {
		allMatches = append(allMatches, url)
	}

	return allMatches
}

func (c *Context) updateWithGame(page *rod.Page, url string) error {

	content := page.MustElement(".content-column")
	lineupsElems := content.MustElements(".combined-lineups-container")
	if len(lineupsElems) != 2 {
		return errors.New("$$$$$***** INVESTIGATE *****$$$$$ " + url)
	}

	allGoals := []db.Goal{}
	allSubs := []db.Substitution{}
	totalTime := 90

	home, goals, subs, totime, err := parseLuElem(lineupsElems[0], true)
	if err != nil {
		return err
	}
	allGoals = append(allGoals, goals...)
	allSubs = append(allSubs, subs...)
	if totime > totalTime {
		totalTime = totime
	}

	away, goals, subs, totime, err := parseLuElem(lineupsElems[0], false)
	if err != nil {
		return err
	}
	allGoals = append(allGoals, goals...)
	allSubs = append(allSubs, subs...)
	if totime > totalTime {
		totalTime = totime
	}

	_, goals, subs, totime, err = parseLuElem(lineupsElems[1], true)
	if err != nil {
		return err
	}
	allGoals = append(allGoals, goals...)
	allSubs = append(allSubs, subs...)
	if totime > totalTime {
		totalTime = totime
	}

	_, goals, subs, totime, err = parseLuElem(lineupsElems[1], false)
	if err != nil {
		return err
	}
	allGoals = append(allGoals, goals...)
	allSubs = append(allSubs, subs...)
	if totime > totalTime {
		totalTime = totime
	}

	sort.Slice(allSubs, func(i, j int) bool {
		return allSubs[i].Min < allSubs[j].Min
	})
	sort.Slice(allGoals, func(i, j int) bool {
		return allGoals[i].Min < allGoals[j].Min
	})

	err = c.MongoClient.InsertGameInfo(c.Competition, url, home, away, allGoals, allSubs)
	if err != nil {
		return errors.Wrap(err, "could not insert game in db")
	}

	i := 0
	lastSub := 0
	for _, sub := range allSubs {
		score := 0
		for i < len(allGoals) && allGoals[i].Min <= sub.Min { // debate if <
			if allGoals[i].Home {
				score++
			} else {
				score--
			}
			i++
		}

		if sub.Min-lastSub > 0 {
			err := c.MongoClient.UpdateScores(home, away, sub.Min-lastSub, float64(score))
			if err != nil {
				return errors.Wrap(err, "cannot update score")
			}
			lastSub = sub.Min
		}
		if sub.Home {
			for j, p := range home {
				if p == sub.Out {
					home[j] = sub.In
				}
			}
		} else {
			for j, p := range away {
				if p == sub.Out {
					away[j] = sub.In
				}
			}
		}
	}

	score := 0
	for i < len(allGoals) {
		if allGoals[i].Home {
			score++
		} else {
			score--
		}
		i++
	}

	return errors.Wrap(c.MongoClient.UpdateScores(home, away, totalTime-lastSub, float64(score)), "cannot update score")
}

func parseLuElem(luElem *rod.Element, home bool) ([]string, []db.Goal, []db.Substitution, int, error) {
	selector := ".right"
	if home {
		selector = ".left"
	}

	lu := make([]string, 0, 12)
	goals := []db.Goal{}
	subs := []db.Substitution{}
	i := 0
	latestEvent := 90

	for _, player := range luElem.MustElement(selector).MustElement("tbody").MustElements("tr") {
		if i > 11 {
			continue
		}

		if b, _, err := player.Has("a"); !b || err != nil {
			continue
		}

		id := player.MustElement("a").MustAttribute("href")
		if id == nil {
			return nil, nil, nil, 0, errors.New("wtf id null for a player")
		}
		lu = append(lu, *id)
		i++

		if ok, _, _ := player.Has(".substitute-out"); ok {
			out := player.MustElement(".substitute-out")
			outElems := strings.Split(out.MustText(), " ")
			min, err := getMinute(outElems[len(outElems)-1])
			if min >= latestEvent {
				latestEvent = min + 1
			}
			if err != nil {
				return nil, nil, nil, 0, errors.Wrap(err, "cannot parse sub time")
			}
			idOut := out.MustElement("a").MustAttribute("href")
			if idOut == nil {
				return nil, nil, nil, 0, errors.New("wtf id null for a player out")
			}
			subs = append(subs, db.Substitution{
				Home: home,
				Min:  min,
				In:   *id,
				Out:  *idOut,
			})
		}

		if ok, _, _ := player.Has(".bookings"); !ok {
			continue
		}

		for _, event := range player.MustElement(".bookings").MustElements("span") {
			img := event.MustElement("img").MustAttribute("src")
			if img == nil {
				continue
			}

			min, err := getMinute(event.MustText())
			if err != nil {
				return nil, nil, nil, 0, errors.Wrap(err, "cannot parse event time")
			}
			if min >= latestEvent {
				latestEvent = min + 1
			}

			if *img == "/media/v1.7.6/img/events/OG.png" {
				goals = append(goals, db.Goal{Home: !home, Min: min})
			}
			if *img == "/media/v1.7.6/img/events/G.png" {
				goals = append(goals, db.Goal{Home: home, Min: min})
			}
			if *img == "/media/v2.7.6/img/events/RC.png" || *img == "/media/v2.7.6/img/events/Y2C.png" {
				subs = append(subs, db.Substitution{
					Home: home, Min: min, Out: *id,
				})
			}
		}
	}

	return lu, goals, subs, latestEvent, nil
}

func getMinute(txt string) (int, error) {
	trimmed := strings.TrimRight(strings.TrimLeft(txt, " "), "'")
	extra := strings.Split(trimmed, "+")
	res := 0
	for _, x := range extra {
		val, err := strconv.Atoi(x)
		if err != nil {
			return 0, err
		}
		res += val
	}
	return res, nil
}
