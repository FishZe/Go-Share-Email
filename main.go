package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/patrickmn/go-cache"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

const (
	SuccessCode        = 0
	EmailNotFount      = 1
	EmailIdNotFount    = 2
	ServerErrorCode    = 4
	AuthorizationError = 5
)

const (
	ServerErrorMsg        = "server error"
	EmailNotFoundMsg      = "email not found"
	EmailIdNotFoundMsg    = "email id not found"
	AuthorizationErrorMsg = "authorization error"
)

type ShareEmailAccount struct {
	EmailId string `json:"email_id"`
	Mails   []Mail `json:"mails"`
}

type QueryRespBody struct {
	Msg        string `json:"msg"`
	Mails      []Mail `json:"mails"`
	EmailId    string `json:"emailId"`
	Email      string `json:"email"`
	UpdateTime int64  `json:"updateTime"`
}

type Config struct {
	Port          int    `yaml:"port"`
	EmailImapHost string `yaml:"email_imap_host"`
	EmailImapPort int    `yaml:"email_imap_port"`
	EmailAccount  string `yaml:"email_account"`
	EmailPassword string `yaml:"email_password"`
	EmailName     string `yaml:"email_name"`
	NeedAuth      bool   `yaml:"need_auth"`
	ClientId      string `yaml:"client_id"`
	ClientSecret  string `yaml:"client_secret"`
	BaseUrl       string `yaml:"base_url"`
}

var mailAccount = cache.New(5*time.Minute, 10*time.Minute)
var queryAccount = cache.New(5*time.Minute, 10*time.Minute)
var email2Id = cache.New(5*time.Minute, 10*time.Minute)
var userLimit = cache.New(1*time.Minute, 1*time.Minute)
var limiter *rate.Limiter
var conf Config

func main() {
	gin.SetMode(gin.ReleaseMode)
	err := errors.New("")
	conf, err = getConf()
	if err != nil {
		makeConfig()
		log.Fatal("请先配置config.yaml文件")
		return
	}
	err = initDB()
	if err != nil {
		log.Fatal(err)
	}
	err = initImap()
	if err != nil {
		log.Printf("init imap failed: %v", err)
		return
	}
	go checkUpdate()
	limiter = rate.NewLimiter(1000, 1000)
	router := gin.Default()
	verify := router.Group("/mail")
	{
		verify.POST("/new", Auth(), MakeNewAccount)
		verify.POST("/query", Auth(), QueryAccount)
	}
	if conf.NeedAuth {
		login := router.Group("/login")
		{
			login.GET("/", LimitRate(), LoginGithub)
			login.GET("/redirect", LimitRate(), RedirectGithub)
		}
	}
	err = router.Run(":" + strconv.Itoa(conf.Port))
}
func LimitRate() gin.HandlerFunc {
	return func(c *gin.Context) {
		ok := limiter.AllowN(time.Now(), 1)
		if ok {
			c.Next()
		} else {
			c.String(http.StatusTooManyRequests, "")
		}
	}
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !conf.NeedAuth {
			c.Next()
			return
		}
		authId := c.Request.Header.Get("Authorization")
		if authId == "" {
			c.JSON(http.StatusOK, gin.H{"code": AuthorizationError, "data": map[string]string{"error": AuthorizationErrorMsg}})
			c.Abort()
			return
		}
		user, err := getUserByUUID(authId)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": ServerErrorCode, "data": map[string]string{"error": ServerErrorMsg}})
			c.Abort()
			return
		}
		if user.NodeId == "" {
			c.JSON(http.StatusOK, gin.H{"code": AuthorizationError, "data": map[string]string{"error": AuthorizationErrorMsg}})
			c.Abort()
			return
		}
		ok := limiter.AllowN(time.Now(), 1)
		if !ok {
			c.String(http.StatusTooManyRequests, "")
			c.Abort()
		}
		lim, found := userLimit.Get(authId)
		if found {
			userLim := lim.(*rate.Limiter)
			ok = userLim.AllowN(time.Now(), 1)
			if ok {
				userLimit.Set(authId, userLim, cache.DefaultExpiration)
				c.Next()
			} else {
				c.String(http.StatusTooManyRequests, "")
				c.Abort()
			}
		} else {
			userLim := rate.NewLimiter(50, 50)
			userLim.AllowN(time.Now(), 1)
			userLimit.Set(authId, userLim, cache.DefaultExpiration)
			c.Next()
		}
	}
}

func makeConfig() {
	f, err := os.Create("./config.yaml")
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Printf("close file failed: %v", err)
		}
	}(f)
	if err != nil {
		log.Printf("create file failed: %v", err)
	} else {
		s, err := yaml.Marshal(&Config{})
		if err != nil {
			log.Printf("marshal config failed: %v", err)
			return
		}
		_, err = f.WriteString(string(s))
	}
}

func getConf() (Config, error) {
	yamlFile, err := os.ReadFile("./config.yaml")
	if err != nil {
		return Config{}, err
	}
	var conf Config
	err = yaml.Unmarshal(yamlFile, &conf)
	if err != nil {
		return Config{}, err
	}
	return conf, nil
}

func randStr(length int) string {
	bytes := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	var result []byte
	rand.Seed(time.Now().UnixNano() + int64(rand.Intn(100)))
	for i := 0; i < length; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return string(result)
}

func getUUID() string {
	id, err := uuid.NewV4()
	if err != nil {
		log.Printf("get uuid failed: %v", err)
		return getUUID()
	}
	return id.String()
}

func MakeNewAccount(c *gin.Context) {
	id := getUUID()
	name := randStr(10) + "@" + conf.EmailName
	queryAccount.Set(id, name, cache.DefaultExpiration)
	email2Id.Set(name, id, cache.DefaultExpiration)
	mailAccount.Set(name, ShareEmailAccount{EmailId: id}, cache.DefaultExpiration)
	c.JSON(http.StatusOK, gin.H{
		"code": SuccessCode,
		"data": gin.H{
			"emailId": id,
			"email":   name,
		},
	})
}

func QueryAccount(c *gin.Context) {
	emailId := c.PostForm("emailId")
	info, found := queryAccount.Get(emailId)
	resp := gin.H{
		"code": SuccessCode,
		"data": QueryRespBody{"", []Mail{}, "", "", 0},
	}
	if !found {
		resp["code"] = EmailIdNotFount
		resp["data"] = QueryRespBody{EmailIdNotFoundMsg, []Mail{}, "", "", 0}
	} else {
		email, found := mailAccount.Get(info.(string))
		if !found {
			resp["code"] = EmailNotFount
			resp["data"] = QueryRespBody{EmailNotFoundMsg, []Mail{}, emailId, "", 0}
		} else {
			queryAccount.Set(emailId, info.(string), cache.DefaultExpiration)
			email2Id.Set(info.(string), emailId, cache.DefaultExpiration)
			mailAccount.Set(info.(string), email.(ShareEmailAccount), cache.DefaultExpiration)
			resp["data"] = QueryRespBody{"", email.(ShareEmailAccount).Mails, emailId, info.(string), mailUpdateTime.Unix()}
		}
	}
	c.JSON(http.StatusOK, resp)
}

func checkUpdate() {
	mailChan := make(chan Mail, 1)
	go func() {
		for mail := range mailChan {
			nowMail, found := mailAccount.Get(mail.To)
			if found {
				id, found := email2Id.Get(mail.To)
				if found {
					newMail := nowMail.(ShareEmailAccount)
					newMail.Mails = append(newMail.Mails, mail)
					sort.Slice(newMail.Mails, func(i, j int) bool {
						return newMail.Mails[i].TimeStamp > newMail.Mails[j].TimeStamp
					})
					if len(newMail.Mails) > 10 {
						newMail.Mails = newMail.Mails[:10]
					}
					mailAccount.Set(mail.To, newMail, cache.DefaultExpiration)
					sqlMail := SQLMail{id.(string), mail.From, mail.TimeStamp, mail.To, mail.Subject}
					insertEmail(sqlMail)
				}
			}
		}
	}()
	for {
		getMessage(mailChan)
		time.Sleep(time.Second * 2)
	}
}
