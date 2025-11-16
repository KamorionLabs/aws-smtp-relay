/*
Package relay provides an interface to relay emails via Amazon SES/Pinpoint API.
*/
package relay

// ARNs holds Amazon Resource Names for cross-account authorization.
type ARNs struct {
	SourceArn     *string
	FromArn       *string
	ReturnPathArn *string
}
