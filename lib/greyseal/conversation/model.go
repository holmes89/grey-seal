package conversation

import (
	"github.com/holmes89/archaea/base"
	. "github.com/holmes89/grey-seal/lib/schemas/greyseal/v1"
)

var _ base.Entity = (*Message)(nil)
var _ base.Repository[*Message] = (MessageRepository)(nil)

var _ base.Entity = (*Conversation)(nil)
var _ base.Repository[*Conversation] = (ConversationRepository)(nil)
