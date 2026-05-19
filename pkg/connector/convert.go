package connector

import (
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/event"
)

func textContent(body string) *event.MessageEventContent {
	html := gfmToMatrixHTML(body)
	return &event.MessageEventContent{
		MsgType:       event.MsgText,
		Body:          body,
		Format:        event.FormatHTML,
		FormattedBody: html,
	}
}

func makeTextEdit(existing []*database.Message, body string) *bridgev2.ConvertedEdit {
	content := textContent(body)
	if len(existing) == 0 {
		return &bridgev2.ConvertedEdit{
			AddedParts: &bridgev2.ConvertedMessage{
				Parts: []*bridgev2.ConvertedMessagePart{{
					Type:    event.EventMessage,
					Content: content,
				}},
			},
		}
	}
	return &bridgev2.ConvertedEdit{
		ModifiedParts: []*bridgev2.ConvertedEditPart{{
			Part:    existing[0],
			Type:    event.EventMessage,
			Content: content,
		}},
	}
}
