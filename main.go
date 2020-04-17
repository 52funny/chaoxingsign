package main

import (
	"bytes"
	"chaoxingsign/course"
	"flag"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// 查询时间
var TIME int

// 开始时间和结素时间
var START, END string

// 黑名单
var BLOCK_LIST map[string]bool

// 账号密码和日志文件地址
var USERNAME, PASSWORD, LOGFILE string

// 签到后老师那里显示的名字
var NAME string

// 签到地址
var ADDRESS string

// 纬度
var LATITUDE string

// 经度
var LONGITUDE string

// 自定图片位置
var PICPATH string

// 模式
var MODEL bool

// Server酱的sckey
var SCKEY string

//初始化读入配置信息
var configpath = flag.String("c", "./config.json", "默认配置文件的地址")

func init() {
	flag.Parse()
	log.SetFlags(log.Ldate | log.Lshortfile)

	file, err := os.Open(*configpath)
	defer file.Close()
	handErr(err)

	bytes, err := ioutil.ReadAll(file)
	handErr(err)

	TIME = int(gjson.GetBytes(bytes, "time").Int())

	START, END = gjson.GetBytes(bytes, "start").String(), gjson.GetBytes(bytes, "end").String()

	BLOCK_LIST = make(map[string]bool)
	list := gjson.GetBytes(bytes, "blockclasslist").Array()
	for _, item := range list {
		BLOCK_LIST[item.String()] = true
	}

	USERNAME, PASSWORD = gjson.GetBytes(bytes, "username").String(), gjson.GetBytes(bytes, "password").String()
	LOGFILE = gjson.GetBytes(bytes, "logfile").String()
	SCKEY = gjson.GetBytes(bytes, "SCKEY").String()
	MODEL = gjson.GetBytes(bytes, "model").Bool()

	fmt.Println(MODEL)

	if MODEL {
		NAME = gjson.GetBytes(bytes, "advance.name").String()
		ADDRESS = gjson.GetBytes(bytes, "advance.address").String()
		LATITUDE = gjson.GetBytes(bytes, "advance.latitude").String()
		LONGITUDE = gjson.GetBytes(bytes, "advance.longitude").String()
		PICPATH = gjson.GetBytes(bytes, "advance.picpath").String()
		//fmt.Println(NAME, ADDRESS, LATITUDE, LONGITUDE, PICPATH)
	}
	//fmt.Println(time.Now())
	//fmt.Println(START)
	//fmt.Println(time.Now().After(START))
}

var Headers = map[string]string{
	"User-Agent": "`Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.108 Safari/537.36`",
}

func main() {
	doSign(USERNAME, PASSWORD)
}

// 主程序
func doSign(username, password string) {
	var CourseData = make([]course.CourseData, 0)
	var All = 0

	// 睡眠时间
	var Sleep = TIME

	var UID string

	cookies := login(username, password)
	UID = getuid(cookies)

	res, err := HandRequest("GET", "http://mooc1-api.chaoxing.com/mycourse/backclazzdata?view=json&rss=1", nil, cookies)
	handErr(err)
	defer res.Body.Close()
	bytes, err := ioutil.ReadAll(res.Body)

	handErr(err)

	if gjson.GetBytes(bytes, "result").String() != "1" {
		fmt.Println("课程列表获取失败！！！！")
		return
	}

	ChannelList := gjson.GetBytes(bytes, "channelList").Array()

	for _, item := range ChannelList {
		//fmt.Println(item)
		// 是否越界
		if len(item.Get("content.course.data").Array()) == 0 {
			continue
		}
		// 处理黑名单
		if contains(BLOCK_LIST, item.Get("content.course.data").Array()[0].Get("name").String()) {
			continue
		}
		CourseData = append(CourseData, course.CourseData{
			CourseId: int(item.Get("content.course.data").Array()[0].Get("id").Int()),
			Name:     item.Get("content.course.data").Array()[0].Get("name").String(),
			ClassId:  int(item.Get("content.id").Int()),
		})
		All++
		

	}
	fmt.Printf("获取成功！！！ \n共%v课程\n", All)

	printCourses(CourseData)
	fl := false

	for {
		start, err := time.ParseInLocation("2006-01-02 15:04:05",
			time.Now().Format("2006-01-02")+" "+
				START,
			time.Local)
		handErr(err)

		end, err := time.ParseInLocation("2006-01-02 15:04:05",
			time.Now().Format("2006-01-02")+" "+
				END,
			time.Local)
		handErr(err)

		if !(time.Now().After(start) && time.Now().Before(end)) {
			continue
		}
		for i := 0; i < All; i++ {
			dealactivity(&fl, UID, CourseData[i], cookies)
			if !fl {
				fmt.Printf("%v [监控运行中]课程: %v 未查询到签到!\n", time.Now().Format("2006-01-02 15:04:05"), CourseData[i].Name)
			} else {
				fl = false
			}
			time.Sleep(time.Duration(Sleep) * time.Second)
		}
	}
}

// 获取上传图片的token
func gettoken(cookie []*http.Cookie) string {
	res, err := HandRequest("GET", "https://pan-yz.chaoxing.com/api/token/uservalid", nil, cookie)
	if err != nil {
		fmt.Println(err)
	}
	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)
	handErr(err)
	return gjson.GetBytes(bytes, "_token").String()
}

// 获取uid
func getuid(cookies []*http.Cookie) string {
	for _, item := range cookies {
		if item.Name == "UID" {
			return item.Value
		}
	}
	return "err"
}

// 上传图片
// references https://studygolang.com/articles/11558
func uploadimage(params map[string]string, cookies []*http.Cookie) string {
	file, err := os.Open(PICPATH)
	handErr(err)
	defer file.Close()
	body := &bytes.Buffer{}
	write := multipart.NewWriter(body)
	part, err := write.CreateFormFile("file", PICPATH)
	handErr(err)
	_, err = io.Copy(part, file)
	handErr(err)
	for k, v := range params {
		_ = write.WriteField(k, v)
	}
	err = write.Close()
	handErr(err)

	req, err := http.NewRequest("POST", "https://pan-yz.chaoxing.com/upload", body)
	req.Header.Set("Content-Type", write.FormDataContentType())

	//add Header
	for k, v := range Headers {
		req.Header.Set(k, v)
	}

	res, err := http.DefaultClient.Do(req)
	handErr(err)
	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)
	handErr(err)
	return gjson.GetBytes(bytes, "objectId").String()
}

//提取activePrimaryId
func getaid(url string) (bool, string) {
	var1 := strings.Split(url[strings.LastIndex(url, "?")+1:], "&")
	for _, it1 := range var1 {
		var2 := strings.Split(it1, "=")
		if var2[0] == "activePrimaryId" {
			return true, var2[1]
		}
	}
	return false, ""
}

// 打印课程信息
func printCourses(c []course.CourseData) {
	for _, item := range c {
		fmt.Printf("所选课程:%v\n", item.Name)
	}
}

func contains(list map[string]bool, query string) bool {
	value, ok := list[query]
	if value && ok {
		return true
	}
	return false
}

//处理activity事项
func dealactivity(flags *bool, UID string, courses course.CourseData, cookies []*http.Cookie) {
	res, err := HandRequest("GET", "https://mobilelearn.chaoxing.com/ppt/activeAPI/taskactivelist?"+
		"courseId="+strconv.Itoa(courses.ClassId)+
		"&classId="+strconv.Itoa(courses.ClassId)+
		"&uid="+UID, nil, cookies)
	handErr(err)
	defer res.Body.Close()

	if res.StatusCode == 200 {

		resbyte, err := ioutil.ReadAll(res.Body)
		//err = ioutil.WriteFile("./gg.json", resbyte, 0666)

		handErr(err)

		//fmt.Println(string(resbyte))

		Activity := gjson.GetBytes(resbyte, "activeList").Array()

		for _, item := range Activity {

			if item.Get("activeType").String() == "2" && item.Get("status").Int() == 1 {
				f, aid := getaid(item.Get("url").String())
				if f {
					fmt.Printf("%v [签到] %v 签到方式:%v 签到状态:%v 签到时间:%v aid:%v\n",
						time.Now().Format("2006-01-02 15:04:05"),
						courses.Name,
						item.Get("nameOne").String(),
						item.Get("nameTwo").String(),
						item.Get("nameFour").String(),
						aid)
					sign(flags, courses.Name, UID, aid, cookies)
				}
			}
		}
	}
}

//签到函数
func sign(flag *bool, coursename, uid, aid string, cookies []*http.Cookie) {
	var res *http.Response
	var err error

	if MODEL {
		// objectId
		objectId := uploadimage(map[string]string{
			"puid":   uid,
			"_token": gettoken(cookies),
		}, cookies)

		res, err = HandRequest("POST", "https://mobilelearn.chaoxing.com/pptSign/stuSignajax", map[string]string{
			"name":      NAME,
			"address":   ADDRESS,
			"activeId":  aid,
			"uid":       uid,
			"longitude": LONGITUDE,
			"latitude":  LATITUDE,
			"objectId":  objectId,
		}, cookies)

	} else {

		urls := "https://mobilelearn.chaoxing.com/pptSign/stuSignajax?activeId=" + aid + "&uid=" + uid + "&clientip=&latitude=-1&longitude=-1&appType=15&fid=0"
		res, err = HandRequest("GET", urls, nil, cookies)

	}
	handErr(err)
	defer res.Body.Close()
	resbyte, err := ioutil.ReadAll(res.Body)
	handErr(err)
	if string(resbyte) == "success" {
		send_sc("签到信息", fmt.Sprintf("```\n课程信息: %v\n签到时间: %v\n签到状态: sucess", coursename, time.Now().Format("2006-01-02 15:04:05")))
		fmt.Println("用户:", uid, "签到成功!")
		*flag = true
	} else {
		fmt.Println("用户:", uid, string(resbyte))
	}
}

// 获取cookies
func login(username, password string) []*http.Cookie {
	urls := "https://passport2-api.chaoxing.com/v11/loginregister"
	res, err := HandRequest("POST", urls, map[string]string{
		"uname": username,
		"code":  password,
	}, nil)
	if err != nil {
		handErr(err)
	}
	defer res.Body.Close()
	return res.Cookies()
}

//Server酱发送信息
func send_sc(title, text string) {
	if SCKEY == "" {
		return
	}
	_, _ = HandRequest("POST", "https://sc.ftqq.com/"+SCKEY+".send", map[string]string{
		"text": title,
		"desp": text,
	}, nil)
}

//网络请求
func HandRequest(method, urls string, maps map[string]string, cookies []*http.Cookie) (*http.Response, error) {
	client := http.Client{}
	values := url.Values{}
	if maps != nil {
		for k, v := range maps {
			values.Add(k, v)
		}
	}
	request, err := http.NewRequest(method, urls, strings.NewReader(values.Encode()))

	if err != nil {
		return nil, err
	}

	// if is post
	if method == "POST" {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	// set other headers
	for k, v := range Headers {
		request.Header.Set(k, v)
	}
	// add cookies
	if cookies != nil {
		for _, v := range cookies {
			request.AddCookie(v)
		}
	}

	return client.Do(request)
}

func handErr(err error) {
	if err != nil {
		file, errs := os.OpenFile(LOGFILE, os.O_CREATE|os.O_RDWR, 0666)
		if errs != nil {
			log.Fatal(err)
		}
		defer file.Close()
		log.SetOutput(file)
		log.Println(err)
		send_sc("Error", err.Error())
		os.Exit(1)
	}
}
