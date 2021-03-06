/*
 * This file is part of "Carnifex"
 * Copyright (C) 2016  Tobias Polzer, Dominik Paulus

 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 * 
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 * 
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301, USA.
 */
package score

import (
	"fmt"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"time"
	"crypto/tls"
)

type JudgeClient struct {
    client http.Client
    username string
    password string
    judge *url.URL
    urls map[APIMethod] *url.URL
}

type APIMethod int8
const (
	JUDGINGS APIMethod = iota
	SUBMISSIONS
	CONTESTS
	CONFIG
	TEAMS
	PROBLEMS
	CATEGORIES
)

func NewJudgeClient(judge *url.URL, username string, password string, insecure bool) *JudgeClient {
	tr := &http.Transport{}
	if(insecure) {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		log.Printf("YOU ARE NOT VERIFYING CERTIFICATES")
	}
    var client = &JudgeClient{
		client: http.Client{Transport: tr},
        judge: judge,
        urls: make(map[APIMethod]*url.URL),
        username: username,
        password: password,
    }
    client.urls[JUDGINGS], _ = judge.Parse("./judgings")
    client.urls[SUBMISSIONS], _ = judge.Parse("./submissions")
    client.urls[CONTESTS], _ = judge.Parse("./contests")
    client.urls[CONFIG], _ = judge.Parse("./config")
    client.urls[TEAMS], _ = judge.Parse("./teams?public=true")
    client.urls[PROBLEMS], _ = judge.Parse("./problems")
	client.urls[CATEGORIES], _ = judge.Parse("./categories?public=true")
    return client
}

func (client *JudgeClient) InjectCid(id int64) {
	for _, m := range []APIMethod{SUBMISSIONS, JUDGINGS, PROBLEMS} {
		u := client.urls[m]
		q := u.Query()
		q.Set("cid", strconv.FormatInt(id, 10))
		u.RawQuery = q.Encode()
	}
}

func (client *JudgeClient) ChannelJson(sink interface{}, method APIMethod, sleep time.Duration, unpack bool) {
	sendType := reflect.TypeOf(sink).Elem()
	recvType := sendType
	if(unpack) {
		recvType = reflect.SliceOf(sendType)
	}
	first := true
	seen := make(map[int64]bool)
	for {
		sv := reflect.New(recvType)
		s := sv.Interface()
		err := client.GetJson(method, s)
		if(err != nil) {
			if(first) {
				log.Fatalf("error while fetching %v: %v", client.urls[method], err)
			}
			log.Print(err)
		} else {
			if(!unpack) {
				reflect.ValueOf(sink).Send(sv.Elem())
			} else {
				slice := sv.Elem()
				for i := 0; i<slice.Len(); i++ {
					v := slice.Index(i)
					id := v.FieldByName("Id").Int()
					sent := seen[id]
					if(!sent) {
						seen[id] = true
						reflect.ValueOf(sink).Send(v)
					}
				}
			}
		}
		first = false
		time.Sleep(sleep)
	}
}

func (client *JudgeClient) GetJson(method APIMethod, p interface{}) (err error){
	url := client.urls[method]
	request, _ := http.NewRequest("", url.String(), nil)
	request.SetBasicAuth(client.username, client.password)
	resp, err := client.client.Do(request)
	if(err != nil) {
		return
	}
	defer resp.Body.Close()
	if(resp.StatusCode != 200) {
		return fmt.Errorf("Got http response \"%s\" while fetching \"%s\"", resp.Status, url)
	}
	var reader io.Reader
	reader = resp.Body
	path := url.Path[1:]
	tmpPath := path + ".tmp"
	if(DumpData) {
		file, err := os.OpenFile(tmpPath, os.O_TRUNC | os.O_CREATE | os.O_WRONLY, 0666)
		if(err != nil) {
			log.Printf("could not dump data: %v", err)
		} else {
			reader = io.TeeReader(reader, file)
			defer file.Close()
		}
	}
	jsonDecoder := json.NewDecoder(reader)
	err = jsonDecoder.Decode(p)
	if(DumpData) {
		os.Rename(tmpPath, path)
	}
	return
}
