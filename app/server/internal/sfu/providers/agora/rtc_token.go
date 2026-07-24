package agora

import rtctokenbuilder "github.com/AgoraIO/Tools/DynamicKey/AgoraDynamicKey/go/src/rtctokenbuilder2"

const tokenTTLSeconds uint32 = 3600

func buildRTCToken(appID, appCertificate, channelName, identity string) (string, error) {
	return rtctokenbuilder.BuildTokenWithUserAccount(
		appID,
		appCertificate,
		channelName,
		identity,
		rtctokenbuilder.RolePublisher,
		tokenTTLSeconds,
		tokenTTLSeconds,
	)
}
