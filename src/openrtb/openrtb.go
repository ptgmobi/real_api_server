package openrtb

// Top level Object
type BidRequest struct {
	Id     string  `json:"id"`               // required. 竞价请求的唯一ID，exchange给出.
	Imps   []*Imp  `json:"imp"`              // required. Imp数组，至少有一个
	Site   *Site   `json:"site,omitempty"`   // recommended.
	App    *App    `json:"app,omitempty"`    // recommended.
	Device *Device `json:"device,omitempty"` // recommended.
	User   *User   `json:"user,omitempty"`   // recommended.
	Test   int     `json:"test"`             // 0: 测试模式，1: 线上模式 默认为0
	At     int     `json:"at"`               // default 2, 1: First Price 2: Second Price Plus
	Tmax   int     `json:"tmax"`             // 接收出价的最长时间(单位：毫秒)
}

// Request source details on post-auction decisioning
// 请求关于拍卖后决策的来源详细信息（如 header bidding）
type Source struct {
}

// Regulatory conditions in effect for all impressions in this bid request.
type Regs struct {
}

// Container for the description of a specific impression; at lest 1 per request.
type Imp struct {
}

// A quantifiable often historical data point about an impression.
type Metric struct {
}

// Details for a banner impression or video companion ad.
type Banner struct {
}

// Details for a vide impression.
type Video struct {
}

// Container for an audio impression.
type Audio struct {
}

// Container for a native impression conforming to the Dynamic Native Ads API.
type Native struct {
}

// An allowed size of a banner.
type Format struct {
}

// Collection of private marketplace(PMP) deals applicable to this impression.
type Pmp struct {
}

// Deal term pertaining to this impression between a seller and buyer.
type Deal struct {
}

// Details of the website calling for the impression.
type Site struct {
}

// Details of the application calling for the impression.
type App struct {
}

// Entity that controls the content of and distributes the site or app.
type Publisher struct {
}

// Details about the published content itself, within which the ad will be shown.
type Content struct {
}

// Producer of the content; not necessarily the publisher
type Producer struct {
}

// Details of the device on which the content and impressions are displayed.
type Device struct {
}

// Location of the device or user's home base depending on the parent object.
type Geo struct {
}

// Human user of the device; audience for advertising.
type User struct {
}

// Collection of additional user targeting data form a specific data source.
type Data struct {
}

// Specific data point about a user from a specific data source.
type Segment struct {
}
