package dooray

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alerting/logging"
	"github.com/grafana/alerting/receivers"
	"github.com/grafana/alerting/templates"
)

func TestNotify(t *testing.T) {
	tests := []struct {
		name            string
		alerts          []*types.Alert
		expectedMessage *doorayMessage
		expectedError   string
		settings        Config
	}{{
		name: "Message is sent",
		settings: Config{
			Url:     "https://example.com/hooks/xxxx",
			Title:   templates.DefaultMessageTitleEmbed,
			IconURL: "",
		},
		alerts: []*types.Alert{{
			Alert: model.Alert{
				Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
				Annotations: model.LabelSet{"ann1": "annv1"},
			},
		}},
		expectedMessage: &doorayMessage{
			BotName: "Grafana",
			Attachments: []attachment{
				{
					Title:     "[FIRING:1]  (val1)",
					TitleLink: "http://localhost/alerting/list",
					Text:      "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
					Color:     "#D63232",
				},
			},
		},
	},
	//{
	//	name: "Message is sent with image URL",
	//	settings: Config{
	//		EndpointURL:    APIURL,
	//		URL:            "https://example.com/hooks/xxxx",
	//		Token:          "",
	//		Recipient:      "#test",
	//		Text:           templates.DefaultMessageEmbed,
	//		Title:          templates.DefaultMessageTitleEmbed,
	//		Username:       "Grafana",
	//		IconEmoji:      ":emoji:",
	//		IconURL:        "",
	//		MentionChannel: "",
	//		MentionUsers:   nil,
	//		MentionGroups:  nil,
	//	},
	//	alerts: []*types.Alert{{
	//		Alert: model.Alert{
	//			Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
	//			Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "image-with-url"},
	//		},
	//	}},
	//	expectedMessage: &slackMessage{
	//		Channel:   "#test",
	//		Username:  "Grafana",
	//		IconEmoji: ":emoji:",
	//		Attachments: []attachment{
	//			{
	//				Title:      "[FIRING:1]  (val1)",
	//				TitleLink:  "http://localhost/alerting/list",
	//				Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
	//				Fallback:   "[FIRING:1]  (val1)",
	//				Fields:     nil,
	//				Footer:     "Grafana v" + appVersion,
	//				FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
	//				Color:      "#D63232",
	//				ImageURL:   "https://www.example.com/test.png",
	//			},
	//		},
	//	},
	//}, {
	//	name: "Message is sent and image on local disk is ignored",
	//	settings: Config{
	//		EndpointURL:    APIURL,
	//		URL:            "https://example.com/hooks/xxxx",
	//		Token:          "",
	//		Recipient:      "#test",
	//		Text:           templates.DefaultMessageEmbed,
	//		Title:          templates.DefaultMessageTitleEmbed,
	//		Username:       "Grafana",
	//		IconEmoji:      ":emoji:",
	//		IconURL:        "",
	//		MentionChannel: "",
	//		MentionUsers:   nil,
	//		MentionGroups:  nil,
	//	},
	//	alerts: []*types.Alert{{
	//		Alert: model.Alert{
	//			Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
	//			Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "image-on-disk"},
	//		},
	//	}},
	//	expectedMessage: &slackMessage{
	//		Channel:   "#test",
	//		Username:  "Grafana",
	//		IconEmoji: ":emoji:",
	//		Attachments: []attachment{
	//			{
	//				Title:      "[FIRING:1]  (val1)",
	//				TitleLink:  "http://localhost/alerting/list",
	//				Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
	//				Fallback:   "[FIRING:1]  (val1)",
	//				Fields:     nil,
	//				Footer:     "Grafana v" + appVersion,
	//				FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
	//				Color:      "#D63232",
	//			},
	//		},
	//	},
	//}
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notifier, recorder, err := setupDoorayForTests(t, test.settings)
			require.NoError(t, err)

			ctx := context.Background()
			ctx = notify.WithGroupKey(ctx, "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			ok, err := notifier.Notify(ctx, test.alerts...)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.False(t, ok)
			} else {
				assert.NoError(t, err)
				assert.True(t, ok)

				// When sending a notification to an Incoming Webhook there should a single request.
				// This is different from PostMessage where some content, such as images, are sent
				// as replies to the original message
				require.Len(t, recorder.requests, 1)

				// Get the request and check that it's sending to the URL of the Incoming Webhook
				r := recorder.requests[0]
				assert.Equal(t, notifier.settings.Url, r.URL.String())

				// Check that the request contains the expected message
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				message := doorayMessage{}
				require.NoError(t, json.Unmarshal(b, &message))
				for i, v := range message.Attachments {
					test.expectedMessage.Attachments[i].Text = v.Text
				}
				assert.Equal(t, *test.expectedMessage, message)
			}
		})
	}
}

//	func TestNotify_PostMessage(t *testing.T) {
//		tests := []struct {
//			name            string
//			alerts          []*types.Alert
//			expectedMessage *doorayMessage
//			expectedReplies []interface{} // can contain either slackMessage or map[string]struct{} for multipart/form-data
//			expectedError   string
//			settings        Config
//		}{{
//			name: "Message is sent",
//			settings: Config{
//				EndpointURL:    APIURL,
//				URL:            APIURL,
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          templates.DefaultMessageTitleEmbed,
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "[FIRING:1]  (val1)",
//						TitleLink:  "http://localhost/alerting/list",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
//						Fallback:   "[FIRING:1]  (val1)",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//		}, {
//			name: "Message is sent with a single alert and a GeneratorURL",
//			settings: Config{
//				EndpointURL:    APIURL,
//				URL:            APIURL,
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          templates.DefaultMessageTitleEmbed,
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations:  model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh"},
//					GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "[FIRING:1]  (val1)",
//						TitleLink:  "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
//						Fallback:   "[FIRING:1]  (val1)",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//		}, {
//			name: "Message is sent with two firing alerts with different GeneratorURLs",
//			settings: Config{
//				EndpointURL:    APIURL,
//				URL:            APIURL,
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          "{{ .Alerts.Firing | len }} firing, {{ .Alerts.Resolved | len }} resolved",
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations:  model.LabelSet{"ann1": "annv1"},
//					GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
//				},
//			}, {
//				Alert: model.Alert{
//					Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
//					Annotations:  model.LabelSet{"ann1": "annv2"},
//					GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-1234567test2",
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "2 firing, 0 resolved",
//						TitleLink:  "http://localhost/alerting/list",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-1234567test2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
//						Fallback:   "2 firing, 0 resolved",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//		}, {
//			name: "Message is sent with two firing alerts with the same GeneratorURLs",
//			settings: Config{
//				EndpointURL:    APIURL,
//				URL:            APIURL,
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          "{{ .Alerts.Firing | len }} firing, {{ .Alerts.Resolved | len }} resolved",
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations:  model.LabelSet{"ann1": "annv1"},
//					GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
//				},
//			}, {
//				Alert: model.Alert{
//					Labels:       model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
//					Annotations:  model.LabelSet{"ann1": "annv2"},
//					GeneratorURL: "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "2 firing, 0 resolved",
//						TitleLink:  "http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSource: http://localhost/alerting/f23a674b-bb6b-46df-8723-12345678test\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
//						Fallback:   "2 firing, 0 resolved",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//		}, {
//			name: "Message is sent with two firing alerts",
//			settings: Config{
//				EndpointURL:    APIURL,
//				URL:            APIURL,
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          "{{ .Alerts.Firing | len }} firing, {{ .Alerts.Resolved | len }} resolved",
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations: model.LabelSet{"ann1": "annv1"},
//				},
//			}, {
//				Alert: model.Alert{
//					Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
//					Annotations: model.LabelSet{"ann1": "annv2"},
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "2 firing, 0 resolved",
//						TitleLink:  "http://localhost/alerting/list",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val2\nAnnotations:\n - ann1 = annv2\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval2\n",
//						Fallback:   "2 firing, 0 resolved",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//		}, {
//			name: "Message is sent and image is uploaded",
//			settings: Config{
//				EndpointURL:    APIURL,
//				URL:            APIURL,
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          templates.DefaultMessageTitleEmbed,
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "image-on-disk"},
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "[FIRING:1]  (val1)",
//						TitleLink:  "http://localhost/alerting/list",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\nDashboard: http://localhost/d/abcd\nPanel: http://localhost/d/abcd?viewPanel=efgh\n",
//						Fallback:   "[FIRING:1]  (val1)",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//			expectedReplies: []interface{}{
//				// check that the following parts are present in the multipart/form-data
//				map[string]struct{}{
//					"file":            {},
//					"channels":        {},
//					"initial_comment": {},
//					"thread_ts":       {},
//				},
//			},
//		}, {
//			name: "Message is sent to custom URL",
//			settings: Config{
//				EndpointURL:    "https://example.com/api",
//				URL:            "https://example.com/api",
//				Token:          "1234",
//				Recipient:      "#test",
//				Text:           templates.DefaultMessageEmbed,
//				Title:          templates.DefaultMessageTitleEmbed,
//				Username:       "Grafana",
//				IconEmoji:      ":emoji:",
//				IconURL:        "",
//				MentionChannel: "",
//				MentionUsers:   nil,
//				MentionGroups:  nil,
//			},
//			alerts: []*types.Alert{{
//				Alert: model.Alert{
//					Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
//					Annotations: model.LabelSet{"ann1": "annv1"},
//				},
//			}},
//			expectedMessage: &slackMessage{
//				Channel:   "#test",
//				Username:  "Grafana",
//				IconEmoji: ":emoji:",
//				Attachments: []attachment{
//					{
//						Title:      "[FIRING:1]  (val1)",
//						TitleLink:  "http://localhost/alerting/list",
//						Text:       "**Firing**\n\nValue: [no value]\nLabels:\n - alertname = alert1\n - lbl1 = val1\nAnnotations:\n - ann1 = annv1\nSilence: http://localhost/alerting/silence/new?alertmanager=grafana&matcher=alertname%3Dalert1&matcher=lbl1%3Dval1\n",
//						Fallback:   "[FIRING:1]  (val1)",
//						Fields:     nil,
//						Footer:     "Grafana v" + appVersion,
//						FooterIcon: "https://grafana.com/static/assets/img/fav32.png",
//						Color:      "#D63232",
//					},
//				},
//			},
//		}}
//
//		for _, test := range tests {
//			t.Run(test.name, func(t *testing.T) {
//				notifier, recorder, err := setupDoorayForTests(t, test.settings)
//				require.NoError(t, err)
//
//				ctx := context.Background()
//				ctx = notify.WithGroupKey(ctx, "alertname")
//				ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})
//
//				ok, err := notifier.Notify(ctx, test.alerts...)
//				if test.expectedError != "" {
//					assert.EqualError(t, err, test.expectedError)
//					assert.False(t, ok)
//				} else {
//					assert.NoError(t, err)
//					assert.True(t, ok)
//
//					// When sending a notification via PostMessage some content, such as images,
//					// are sent as replies to the original message
//					require.Len(t, recorder.requests, len(test.expectedReplies)+1)
//
//					// Get the request and check that it's sending to the URL
//					r := recorder.requests[0]
//					assert.Equal(t, notifier.settings.URL, r.URL.String())
//
//					// Check that the request contains the expected message
//					b, err := io.ReadAll(r.Body)
//					require.NoError(t, err)
//
//					message := slackMessage{}
//					require.NoError(t, json.Unmarshal(b, &message))
//					for i, v := range message.Attachments {
//						// Need to update the ts as these cannot be set in the test definition
//						test.expectedMessage.Attachments[i].Ts = v.Ts
//					}
//					assert.Equal(t, *test.expectedMessage, message)
//
//					// Check that the replies match expectations
//					for i := 1; i < len(recorder.requests); i++ {
//						r = recorder.requests[i]
//						assert.Equal(t, "https://slack.com/api/files.upload", r.URL.String())
//
//						media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
//						require.NoError(t, err)
//						if media == "multipart/form-data" {
//							// Some replies are file uploads, so check the multipart form
//							checkMultipart(t, test.expectedReplies[i-1].(map[string]struct{}), r.Body, params["boundary"])
//						} else {
//							b, err = io.ReadAll(r.Body)
//							require.NoError(t, err)
//							message = slackMessage{}
//							require.NoError(t, json.Unmarshal(b, &message))
//							assert.Equal(t, test.expectedReplies[i-1], message)
//						}
//					}
//				}
//			})
//		}
//	}
//
// doorayRequestRecorder is used in tests to record all requests.
type doorayRequestRecorder struct {
	requests []*http.Request
}

func (s *doorayRequestRecorder) fn(_ context.Context, r *http.Request, _ logging.Logger) (string, error) {
	s.requests = append(s.requests, r)
	return "", nil
}

// // checkMulipart checks that each part is present, but not its contents
//
//	func checkMultipart(t *testing.T, expected map[string]struct{}, r io.Reader, boundary string) {
//		m := multipart.NewReader(r, boundary)
//		visited := make(map[string]struct{})
//		for {
//			part, err := m.NextPart()
//			if errors.Is(err, io.EOF) {
//				break
//			}
//			require.NoError(t, err)
//			visited[part.FormName()] = struct{}{}
//		}
//		assert.Equal(t, expected, visited)
//	}
func setupDoorayForTests(t *testing.T, settings Config) (*Notifier, *doorayRequestRecorder, error) {
	tmpl := templates.ForTests(t)
	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	f, err := os.Create(t.TempDir() + "test.png")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = f.Close()
		if err := os.Remove(f.Name()); err != nil {
			t.Logf("failed to delete test file: %s", err)
		}
	})

	notificationService := receivers.MockNotificationService()

	sn := &Notifier{
		Base: &receivers.Base{
			Name:                  "",
			Type:                  "",
			UID:                   "",
			DisableResolveMessage: false,
		},
		log:      &logging.FakeLogger{},
		ns:       notificationService,
		tmpl:     tmpl,
		settings: settings,
	}

	sr := &doorayRequestRecorder{}
	sn.sendFn = sr.fn
	return sn, sr, nil
}
