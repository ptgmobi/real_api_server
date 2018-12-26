package aes

import (
	"testing"
)

func tt(text string) bool {
	cipher := Encrypt(text)
	newText := Decrypt(cipher)
	return text == newText
}

func TestInitKeyHexError(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("shoud panic")
		}
	}()
	cf := &Conf{
		Key: "XXYYZZ4455667788112233445566778811223344556677881122334455667788",
	}
	Init(cf)
}

func TestInitKeySize(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("shoud panic")
		}
	}()
	cf := &Conf{
		// Key: "1122334455667788112233445566778811223344556677881122334455667788",
		Key: "11223344556677881122334455667788112233445566778811223344556677", // missing the last 88
	}
	Init(cf)
}

func TestAes(t *testing.T) {
	cf := &Conf{
		Key: "1122334455667788112233445566778811223344556677881122334455667788",
	}
	Init(cf)
	ok := true
	ok = ok && tt("hello world")
	ok = ok && tt("日月忽其不淹兮，春与秋其代序。惟草木之零落兮，恐美人之迟暮。")
	ok = ok && tt("蒹葭苍苍，白露为霜，所谓伊人，在水一方")
	ok = ok && tt("【AbemaTVとは】会員数4000万人超のAmeba（アメーバ）を運営するサイバーエージェントとテレビ朝日が共同で、“インターネットテレビ局”として展開する、新たな動画配信事業です。2016年4月11日（月）に本開局を予定しており、本開局以降はオリジナルの生放送コンテンツや、ニュース、音楽、スポーツなど多彩な番組が楽しめる約20チャンネルをすべて無料で提供します。【一部先行配信チャンネル】・Ab")

	if !ok {
		t.Error("test fail")
	}
}
