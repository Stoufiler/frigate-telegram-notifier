import (
	"embed" // New import
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*
var content embed.FS

var T *i18n.Localizer

// i18n Bundle
var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	// Charger les fichiers de traduction avec gestion d'erreur
	// Utilisation de embed.FS pour charger depuis le binaire
	_, err := bundle.LoadMessageFileFS(content, "locales/en.json")
	if err != nil {
		panic(fmt.Sprintf("Erreur chargement locales/en.json: %v", err))
	}
	_, err = bundle.LoadMessageFileFS(content, "locales/fr.json")
	if err != nil {
		panic(fmt.Sprintf("Erreur chargement locales/fr.json: %v", err))
	}
}

func main() {
	// Charger la configuration
	lang := os.Getenv("BOT_LANGUAGE")
	if lang == "" {
		lang = "fr" // Langue par défaut
	}
	T = i18n.NewLocalizer(bundle, lang)

	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	mqttBroker := os.Getenv("MQTT_BROKER")
	mqttUser := os.Getenv("MQTT_USERNAME")
	mqttPass := os.Getenv("MQTT_PASSWORD")
	mqttTopic := os.Getenv("MQTT_TOPIC")
	frigateURL := os.Getenv("FRIGATE_URL")
	cameraList := os.Getenv("CAMERA_LIST")
	objectFilterStr := os.Getenv("MQTT_OBJECT_FILTER")

	if telegramToken == "" || chatIDStr == "" || mqttBroker == "" || mqttTopic == "" || frigateURL == "" {
		panic("⚠️ Il manque des variables d'environnement obligatoires")
	}

	chatID, err := parseChatID(chatIDStr)
	if err != nil {
		panic(fmt.Sprintf("⚠️ CHAT_ID invalide: %v", err))
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		panic(fmt.Sprintf("⚠️ Impossible de créer le bot Telegram: %v", err))
	}
	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "bot_ready"}))
	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "test_template", TemplateData: map[string]interface{}{"TestVar": "une variable de test"}}))

	// Map des caméras autorisées
	allowedCameras := map[string]bool{}
	if cameraList != "" {
		for _, cam := range strings.Split(cameraList, ",") {
			allowedCameras[strings.TrimSpace(cam)] = true
		}
	}

	// Map des objets autorisés pour la notification
	objectFilter := map[string]bool{}
	if objectFilterStr != "" {
		fmt.Println("✅ Filtre d'objets activé pour:", objectFilterStr)
		for _, obj := range strings.Split(objectFilterStr, ",") {
			objectFilter[strings.TrimSpace(obj)] = true
		}
	}

	// Connexion MQTT
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker)
	if mqttUser != "" && mqttPass != "" {
		opts.SetUsername(strings.TrimSpace(mqttUser))
		opts.SetPassword(strings.TrimSpace(mqttPass))
	}
	opts.OnConnect = func(client mqtt.Client) {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "mqtt_connected"}))
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "mqtt_connection_lost", TemplateData: map[string]interface{}{"Error": err}}))
	}

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "mqtt_subscription_failed", TemplateData: map[string]interface{}{"Error": token.Error()}}))
	}

	// Abonnement
	if token := client.Subscribe(mqttTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		var evt Event
		if err := json.Unmarshal(m.Payload(), &evt); err != nil {
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "json_error", TemplateData: map[string]interface{}{"Error": err}}))
			return
		}

		// On traite les nouveaux événements (snapshot) et la fin (clip vidéo)
		if evt.Type == "new" {
			// Logique pour envoyer le snapshot au début de l'événement
			handleNewEvent(bot, chatID, frigateURL, evt, allowedCameras, objectFilter)
		} else if evt.Type == "end" {
			// Logique pour envoyer le clip vidéo à la fin de l'événement
			handleEndEvent(bot, chatID, frigateURL, evt, allowedCameras, objectFilter)
		}
	}); token.Wait() && token.Error() != nil {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "mqtt_subscription_failed", TemplateData: map[string]interface{}{"Error": token.Error()}}))
	}

	// Message de démarrage
	startMsg := tgbotapi.NewMessage(chatID, T.MustLocalize(&i18n.LocalizeConfig{MessageID: "telegram_start_message"}))
	if _, err := bot.Send(startMsg); err != nil {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "telegram_start_failed", TemplateData: map[string]interface{}{"Error": err}}))
	} else {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "preview_sent"}))
	}

	select {}
}

func parseChatID(id string) (int64, error) {
	chatID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("chat ID invalide: %w", err)
	}
	return chatID, nil
}

// translateLabel traduit une étiquette de l'anglais vers le français.
func translateLabel(label string) string {
	return T.MustLocalize(&i18n.LocalizeConfig{MessageID: "label_" + label})
}

func handleNewEvent(bot *tgbotapi.BotAPI, chatID int64, frigateURL string, evt Event, allowedCameras map[string]bool, objectFilter map[string]bool) {
	camName := evt.After.Camera
	if len(allowedCameras) > 0 && !allowedCameras[camName] {
		return // Ignorer si la caméra n'est pas dans la liste
	}

	// Appliquer le filtre d'objets
	if len(objectFilter) > 0 {
		found := false
		for _, obj := range evt.After.Data.Objects {
			if objectFilter[obj] {
				found = true
				break
			}
		}
		if !found {
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "object_ignored", TemplateData: map[string]interface{}{"Objects": strings.Join(evt.After.Data.Objects, ", ")}}))
			return
		}
	}

	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "movement_detected", TemplateData: map[string]interface{}{"CameraName": camName}}))

	if len(evt.After.Data.Detections) == 0 {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "no_detection_id"}))
		return
	}
	detectionID := evt.After.Data.Detections[0]
	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "detection_id_extracted", TemplateData: map[string]interface{}{"DetectionID": detectionID}}))

	var resp *http.Response
	var err error
	maxRetries := 5
	retryDelay := 2 * time.Second

	imgURL := fmt.Sprintf("%s/api/events/%s/snapshot.jpg?bbox=1", frigateURL, detectionID)
	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "downloading_snapshot", TemplateData: map[string]interface{}{"URL": imgURL}}))

	for i := 0; i < maxRetries; i++ {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "snapshot_retry_attempt", TemplateData: map[string]interface{}{"Attempt": i + 1, "MaxRetries": maxRetries}}))
		resp, err = http.Get(imgURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "snapshot_success"}))
			break
		}
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "snapshot_failed", TemplateData: map[string]interface{}{"Attempt": i + 1, "MaxRetries": maxRetries, "Status": resp.Status, "Body": string(body)}}))
		} else {
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "snapshot_get_error", TemplateData: map[string]interface{}{"Attempt": i + 1, "MaxRetries": maxRetries, "Error": err}}))
		}
		time.Sleep(retryDelay)
	}

	if resp == nil || resp.StatusCode != http.StatusOK {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "snapshot_download_failed"}))
		return
	}
	defer resp.Body.Close()

	tmpFile := "/tmp/" + detectionID + ".jpg"
	file, _ := os.Create(tmpFile)
	defer file.Close()
	defer os.Remove(tmpFile)
	io.Copy(file, resp.Body)

	// Construire une légende enrichie
	caption := T.MustLocalize(&i18n.LocalizeConfig{MessageID: "caption_snapshot_simple", TemplateData: map[string]interface{}{"CameraName": camName}})
	if len(evt.After.Data.Objects) > 0 {
		translatedObjects := []string{}
		for _, obj := range evt.After.Data.Objects {
			translatedObjects = append(translatedObjects, translateLabel(obj))
		}
		caption = T.MustLocalize(&i18n.LocalizeConfig{MessageID: "caption_snapshot_object", TemplateData: map[string]interface{}{"Objects": strings.Title(strings.Join(translatedObjects, ", ")), "CameraName": camName}})
	}
	if len(evt.After.Data.Zones) > 0 {
		caption += T.MustLocalize(&i18n.LocalizeConfig{MessageID: "caption_snapshot_zone", TemplateData: map[string]interface{}{"Zones": strings.Join(evt.After.Data.Zones, ", ")}})
	}

	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(tmpFile))
	photo.Caption = caption

	if _, err := bot.Send(photo); err != nil {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "telegram_send_photo_error", TemplateData: map[string]interface{}{"Error": err}}))
	} else {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "snapshot_sent", TemplateData: map[string]interface{}{"CameraName": camName}}))
	}
}

func handleEndEvent(bot *tgbotapi.BotAPI, chatID int64, frigateURL string, evt Event, allowedCameras map[string]bool, objectFilter map[string]bool) {
	camName := evt.After.Camera
	if len(allowedCameras) > 0 && !allowedCameras[camName] {
		return // Ignorer si la caméra n'est pas dans la liste
	}

	// Appliquer le filtre d'objets
	if len(objectFilter) > 0 {
		found := false
		for _, obj := range evt.After.Data.Objects {
			if objectFilter[obj] {
				found = true
				break
			}
		}
		if !found {
			// Pas besoin de log ici, car l'événement 'new' a déjà été ignoré
			return
		}
	}

	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "end_of_event", TemplateData: map[string]interface{}{"CameraName": camName}}))

	// Pour l'aperçu, nous utilisons l'ID de la "review"
	reviewID := evt.After.ID
	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "review_id_extracted", TemplateData: map[string]interface{}{"ReviewID": reviewID}}))

	previewURL := fmt.Sprintf("%s/api/review/%s/preview", frigateURL, reviewID)
	fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "downloading_preview", TemplateData: map[string]interface{}{"URL": previewURL}}))

	resp, err := http.Get(previewURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "preview_download_failed", TemplateData: map[string]interface{}{"Status": resp.Status, "Body": string(body)}}))
		} else {
			fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "preview_get_error", TemplateData: map[string]interface{}{"Error": err}}))
		}
		return
	}
	defer resp.Body.Close()

	// On suppose que l'aperçu est un GIF
	tmpFile := "/tmp/" + reviewID + ".gif"
	file, _ := os.Create(tmpFile)
	defer file.Close()
	defer os.Remove(tmpFile)
	io.Copy(file, resp.Body)

	// Construire une légende enrichie
	duration := T.MustLocalize(&i18n.LocalizeConfig{MessageID: "unknown_duration"})
	if evt.After.EndTime != nil {
		duration = fmt.Sprintf("%.0f secondes", *evt.After.EndTime-evt.After.StartTime)
	}
	caption := T.MustLocalize(&i18n.LocalizeConfig{MessageID: "caption_preview_simple", TemplateData: map[string]interface{}{"CameraName": camName, "Duration": duration}})
	if len(evt.After.Data.Objects) > 0 {
		translatedObjects := []string{}
		for _, obj := range evt.After.Data.Objects {
			translatedObjects = append(translatedObjects, translateLabel(obj))
		}
		caption = T.MustLocalize(&i18n.LocalizeConfig{MessageID: "caption_preview_object", TemplateData: map[string]interface{}{"Objects": strings.Title(strings.Join(translatedObjects, ", ")), "CameraName": camName, "Duration": duration}})
	}
	if len(evt.After.Data.Zones) > 0 {
		caption += T.MustLocalize(&i18n.LocalizeConfig{MessageID: "caption_preview_zone", TemplateData: map[string]interface{}{"Zones": strings.Join(evt.After.Data.Zones, ", ")}})
	}

	// Envoyer en tant qu'animation (GIF)
	animation := tgbotapi.NewAnimation(chatID, tgbotapi.FilePath(tmpFile))
	animation.Caption = caption

	if _, err := bot.Send(animation); err != nil {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "telegram_send_animation_error", TemplateData: map[string]interface{}{"Error": err}}))
	} else {
		fmt.Println(T.MustLocalize(&i18n.LocalizeConfig{MessageID: "preview_sent", TemplateData: map[string]interface{}{"CameraName": camName}}))
	}
}