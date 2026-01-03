package core

type AIConversation struct {
	ID int
	Context *ConversationContext
}

// TODO: The intetent mechanism might want this AIConversation as well! maybe, unless we want to limit context
