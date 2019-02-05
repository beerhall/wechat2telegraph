package main

import (
	"bufio"
	"crypto/sha512"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"os"
	"os/exec"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/PuerkitoBio/goquery"
	telegraph "github.com/beerhall/telegraph-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"
)

func main() {
	viper.SetConfigName("config")                  // name of config file (without extension)
	viper.AddConfigPath("/root/wechat2telegraph/") // path to look for the config file in
	err := viper.ReadInConfig()                    // Find and read the config file
	if err != nil {                                // Handle errors reading the config file
		log.Panicf("Fatal error config file: %s \n", err)
	}

	token := viper.Get("token").(string)
	address := viper.Get("address").(string)
	port := viper.Get("port").(string)
	certFile := viper.Get("cert_file").(string)
	keyFile := viper.Get("key_file").(string)
	websiteFolder := viper.Get("website_folder").(string)
	imageFolder := viper.Get("image_folder").(string)
	logFile, _ := os.OpenFile(viper.Get("log_file").(string), os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	log.SetOutput(logFile)

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	_, err = bot.SetWebhook(tgbotapi.NewWebhookWithCert("https://"+address+":"+port+"/"+bot.Token, certFile))
	if err != nil {
		log.Fatal(err)
	}
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	if info.LastErrorDate != 0 {
		log.Error("Telegram callback failed: %s", info.LastErrorMessage)
	}
	updates := bot.ListenForWebhook("/" + bot.Token)
	go http.ListenAndServeTLS("0.0.0.0:"+port, certFile, keyFile, nil)

	for update := range updates {
		if u, err := url.Parse(update.Message.Text); (err == nil) && (u.Hostname() == "mp.weixin.qq.com") {
			// Request the HTML page.
			res, err := http.Get(update.Message.Text)
			if err != nil {
				log.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
			}

			// Load the HTML document
			doc, err := goquery.NewDocumentFromReader(res.Body)
			if err != nil {
				log.Fatal(err)
			}

			doc.Find("img").Each(func(i int, s *goquery.Selection) {
				data_src, _ := s.Attr("data-src")
				filename := fmt.Sprintf("%x", sha512.Sum512([]byte(data_src)))[0:12]

				cmd := exec.Command("curl", "-o", websiteFolder+imageFolder+filename, data_src)
				log.Info("Running command and waiting for it to finish...")
				cmd.Run()

				f, _ := os.Open(websiteFolder + imageFolder + filename)
				r := bufio.NewReader(f)
				_, format, _ := image.DecodeConfig(r)

				os.Rename(websiteFolder+imageFolder+filename, websiteFolder+imageFolder+filename+"."+format)
				s.SetAttr("src", "https://"+address+"/"+imageFolder+filename+"."+format)
				log.Info("filename:\t" + filename + "." + format)
			})

			title, _ := doc.Find("#activity-name").First().Html()
			author, _ := doc.Find("#js_name").First().Html()
			html, _ := doc.Find("#js_content").First().Html()

			if client, err := telegraph.Create(author, author, ""); err == nil {
				log.Info("> Created client: %#+v", client)

				// GetAccountInfo
				if account, err := client.GetAccountInfo(nil); err == nil {
					log.Info("> GetAccountInfo result: %#+v", account)
				} else {
					log.Info("* GetAccountInfo error: %s", err)
				}

				// CreatePage
				if page, err := client.CreatePageWithHTML(title, author, "", html, false); err == nil {
					log.Info("> CreatePage result: %#+v", page)
					log.Info("> Created page url: %s", page.URL)

					msg := tgbotapi.NewMessage(update.Message.Chat.ID, page.URL)

					bot.Send(msg)

				} else {
					log.Error("* CreatePage error: %s", err)
				}
			}
		} else {
			log.Errorf("Not URL or Host name error: %s", err)
		}
	}
}
