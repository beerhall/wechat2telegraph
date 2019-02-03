package main

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"
	telegraph "github.com/beerhall/telegraph-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

	"log"
)

func main() {
	token := "771387478:AAFb_szalVh_LekBLDZRdXeuBlFSfn_ytFc"
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	_, err = bot.SetWebhook(tgbotapi.NewWebhookWithCert("https://108.61.162.7:8443/"+bot.Token, "/root/wechat2telegraph/wechat2telegraph.pem"))
	if err != nil {
		log.Fatal(err)
	}
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}
	updates := bot.ListenForWebhook("/" + bot.Token)
	go http.ListenAndServeTLS("0.0.0.0:8443", "/root/wechat2telegraph/wechat2telegraph.pem", "/root/wechat2telegraph/wechat2telegraph.key", nil)

	for update := range updates {
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

		title, _ := doc.Find("#activity-name").First().Html()
		author, _ := doc.Find(".rich_media_meta.rich_media_meta_text").First().Html()
		html, _ := doc.Find("#js_content").First().Html()

		if client, err := telegraph.Create(author, author, ""); err == nil {
			log.Printf("> Created client: %#+v", client)

			// GetAccountInfo
			if account, err := client.GetAccountInfo(nil); err == nil {
				log.Printf("> GetAccountInfo result: %#+v", account)
			} else {
				log.Printf("* GetAccountInfo error: %s", err)
			}

			// CreatePage
			if page, err := client.CreatePageWithHTML(title, author, "", html, true); err == nil {
				log.Printf("> CreatePage result: %#+v", page)
				log.Printf("> Created page url: %s", page.URL)

				// GetPage
				if page, err := client.GetPage(page.Path, true); err == nil {
					log.Printf("> GetPage result: %#+v", page)
				} else {
					log.Printf("* GetPage error: %s", err)
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, page.URL)

				bot.Send(msg)

			} else {
				log.Printf("* CreatePage error: %s", err)
			}
		}
	}
}
