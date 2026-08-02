package models

import (
	"time"

	"gorm.io/gorm"
)

type Telegraph struct {
	gorm.Model
	Time            string          `json:"time"`
	DataTime        *time.Time      `json:"dataTime" gorm:"index" wails:"-"`
	Title           string          `json:"title" gorm:"index"`
	Content         string          `json:"content" gorm:"index"`
	SubjectTags     []string        `json:"subjects" gorm:"-:all"`
	StocksTags      []string        `json:"stocks" gorm:"-:all"`
	IsRed           bool            `json:"isRed" gorm:"index"`
	Url             string          `json:"url"`
	Source          string          `json:"source" gorm:"index"`
	TelegraphTags   []TelegraphTags `json:"tags" gorm:"-:migration;foreignKey:TelegraphId"`
	SentimentResult string          `json:"sentimentResult" gorm:"index"`
}
type TelegraphTags struct {
	gorm.Model
	TagId       uint `json:"tagId"`
	TelegraphId uint `json:"telegraphId"`
}

func (t TelegraphTags) TableName() string {
	return "telegraph_tags"
}

type Tags struct {
	gorm.Model
	Name string `json:"name"`
	Type string `json:"type"`
}

func (p Tags) TableName() string {
	return "tags"
}

func (p Telegraph) TableName() string {
	return "telegraph_list"
}

type TVNews struct {
	Id         string `json:"id"`
	Title      string `json:"title"`
	Published  int    `json:"published"`
	Urgency    int    `json:"urgency"`
	Permission string `json:"permission"`
	StoryPath  string `json:"storyPath"`
	Provider   struct {
		Id     string `json:"id"`
		Name   string `json:"name"`
		LogoId string `json:"logo_id"`
	} `json:"provider"`
}
type TVNewsDetail struct {
	ShortDescription string `json:"shortDescription"`
	Tags             []struct {
		Title string `json:"title"`
		Args  []struct {
			Id    string `json:"id"`
			Value string `json:"value"`
		} `json:"args"`
	} `json:"tags"`
	Copyright string `json:"copyright"`
	Id        string `json:"id"`
	Title     string `json:"title"`
	Published int    `json:"published"`
	Urgency   int    `json:"urgency"`
	StoryPath string `json:"storyPath"`
}

type ReutersNews struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Result     struct {
		ParentSectionName string `json:"parent_section_name"`
		Pagination        struct {
			Size         int    `json:"size"`
			ExpectedSize int    `json:"expected_size"`
			TotalSize    int    `json:"total_size"`
			Orderby      string `json:"orderby"`
		} `json:"pagination"`
		DateModified time.Time `json:"date_modified" wails:"-"`
		FetchType    string    `json:"fetch_type"`
		Articles     []struct {
			Id                          string    `json:"id"`
			CanonicalUrl                string    `json:"canonical_url"`
			Website                     string    `json:"website"`
			Web                         string    `json:"web"`
			Native                      string    `json:"native"`
			UpdatedTime                 time.Time `json:"updated_time" wails:"-"`
			PublishedTime               time.Time `json:"published_time" wails:"-"`
			ArticleType                 string    `json:"article_type"`
			DisplayMyNews               bool      `json:"display_my_news"`
			DisplayNewsletterSignup     bool      `json:"display_newsletter_signup"`
			DisplayNotifications        bool      `json:"display_notifications"`
			DisplayRelatedMedia         bool      `json:"display_related_media"`
			DisplayRelatedOrganizations bool      `json:"display_related_organizations"`
			ContentCode                 string    `json:"content_code"`
			Source                      struct {
				Name         string `json:"name"`
				OriginalName string `json:"original_name"`
			} `json:"source"`
			Title            string `json:"title"`
			BasicHeadline    string `json:"basic_headline"`
			Distributor      string `json:"distributor"`
			Description      string `json:"description"`
			PrimaryMediaType string `json:"primary_media_type,omitempty"`
			PrimaryTag       struct {
				ShortBio    string `json:"short_bio"`
				Description string `json:"description"`
				Slug        string `json:"slug"`
				Text        string `json:"text"`
				TopicUrl    string `json:"topic_url"`
				CanFollow   bool   `json:"can_follow,omitempty"`
				IsTopic     bool   `json:"is_topic,omitempty"`
			} `json:"primary_tag"`
			WordCount   int `json:"word_count"`
			ReadMinutes int `json:"read_minutes"`
			Kicker      struct {
				Path  string   `json:"path"`
				Names []string `json:"names"`
				Name  string   `json:"name,omitempty"`
			} `json:"kicker"`
			AdTopics  []string `json:"ad_topics"`
			Thumbnail struct {
				Url                   string    `json:"url"`
				Caption               string    `json:"caption,omitempty"`
				Type                  string    `json:"type"`
				ResizerUrl            string    `json:"resizer_url"`
				Location              string    `json:"location,omitempty"`
				Id                    string    `json:"id"`
				Authors               string    `json:"authors,omitempty"`
				AltText               string    `json:"alt_text"`
				Width                 int       `json:"width"`
				Height                int       `json:"height"`
				Subtitle              string    `json:"subtitle"`
				Slug                  string    `json:"slug,omitempty"`
				UpdatedAt             time.Time `json:"updated_at" wails:"-"`
				Company               string    `json:"company,omitempty"`
				PurchaseLicensingPath string    `json:"purchase_licensing_path,omitempty"`
			} `json:"thumbnail"`
			Authors []struct {
				Id        string `json:"id,omitempty"`
				Name      string `json:"name"`
				FirstName string `json:"first_name,omitempty"`
				LastName  string `json:"last_name,omitempty"`
				Company   string `json:"company"`
				Thumbnail struct {
					Url        string `json:"url"`
					Type       string `json:"type"`
					ResizerUrl string `json:"resizer_url"`
				} `json:"thumbnail"`
				SocialLinks []struct {
					Site string `json:"site"`
					Url  string `json:"url"`
				} `json:"social_links,omitempty"`
				Byline      string `json:"byline"`
				Description string `json:"description,omitempty"`
				TopicUrl    string `json:"topic_url,omitempty"`
				Role        string `json:"role,omitempty"`
			} `json:"authors"`
			DisplayTime   time.Time `json:"display_time" wails:"-"`
			ThumbnailDark struct {
				Url        string    `json:"url"`
				Type       string    `json:"type"`
				ResizerUrl string    `json:"resizer_url"`
				Id         string    `json:"id"`
				AltText    string    `json:"alt_text"`
				Width      int       `json:"width"`
				Height     int       `json:"height"`
				Subtitle   string    `json:"subtitle"`
				UpdatedAt  time.Time `json:"updated_at" wails:"-"`
			} `json:"thumbnail_dark,omitempty"`
		} `json:"articles"`
		Section struct {
			Id          string `json:"id"`
			AdUnitCode  string `json:"ad_unit_code"`
			Website     string `json:"website"`
			Name        string `json:"name"`
			PageTitle   string `json:"page_title"`
			CanFollow   bool   `json:"can_follow"`
			Language    string `json:"language"`
			Type        string `json:"type"`
			Advertising struct {
				Sponsored string `json:"sponsored"`
			} `json:"advertising"`
			VideoPlaylistId  string `json:"video_playlistId"`
			MobileAdUnitPath string `json:"mobile_ad_unit_path"`
			AdUnitPath       string `json:"ad_unit_path"`
			CollectionAlias  string `json:"collection_alias"`
			SectionAbout     string `json:"section_about"`
			Title            string `json:"title"`
			Personalization  struct {
				Id        string `json:"id"`
				Type      string `json:"type"`
				ShowTags  bool   `json:"show_tags"`
				CanFollow bool   `json:"can_follow"`
			} `json:"personalization"`
		} `json:"section"`
		AdUnitPath   string `json:"ad_unit_path"`
		ResponseTime int64  `json:"response_time"`
	} `json:"result"`
	Id string `json:"_id"`
}

type CailianpressWeb struct {
	Total int `json:"total"`
	List  []struct {
		Title   string `json:"title" md:"资讯标题"`
		Ctime   int    `json:"ctime" md:"资讯时间"`
		Content string `json:"content" md:"资讯内容"`
		Author  string `json:"author" md:"资讯发布者"`
	} `json:"list"`
}

type InteractiveAnswer struct {
	PageNo      int                        `json:"pageNo"`
	PageSize    int                        `json:"pageSize"`
	TotalRecord int                        `json:"totalRecord"`
	TotalPage   int                        `json:"totalPage"`
	Results     []InteractiveAnswerResults `json:"results"`
	Count       bool                       `json:"count"`
}

type InteractiveAnswerResults struct {
	EsId             string   `json:"esId" md:"-"`
	IndexId          string   `json:"indexId" md:"-"`
	ContentType      int      `json:"contentType" md:"-"`
	Trade            []string `json:"trade"  md:"行业名称"`
	MainContent      string   `json:"mainContent" md:"投资者提问"`
	StockCode        string   `json:"stockCode" md:"股票代码"`
	Secid            string   `json:"secid" md:"-"`
	CompanyShortName string   `json:"companyShortName" md:"股票名称"`
	CompanyLogo      string   `json:"companyLogo,omitempty" md:"-"`
	BoardType        []string `json:"boardType" md:"-"`
	PubDate          string   `json:"pubDate" md:"发布时间"`
	UpdateDate       string   `json:"updateDate" md:"-"`
	Author           string   `json:"author" md:"-"`
	AuthorName       string   `json:"authorName" md:"-"`
	PubClient        string   `json:"pubClient" md:"-"`
	AttachedId       string   `json:"attachedId" md:"-"`
	AttachedContent  string   `json:"attachedContent" md:"上市公司回复"`
	AttachedAuthor   string   `json:"attachedAuthor" md:"-"`
	AttachedPubDate  string   `json:"attachedPubDate" md:"回复时间"`
	Score            float64  `json:"score" md:"-"`
	TopStatus        int      `json:"topStatus" md:"-"`
	PraiseCount      int      `json:"praiseCount" md:"-"`
	PraiseStatus     bool     `json:"praiseStatus" md:"-"`
	FavoriteStatus   bool     `json:"favoriteStatus" md:"-"`
	AttentionCompany bool     `json:"attentionCompany" md:"-"`
	IsCheck          string   `json:"isCheck" md:"-"`
	QaStatus         int      `json:"qaStatus" md:"-"`
	PackageDate      string   `json:"packageDate" md:"-"`
	RemindStatus     bool     `json:"remindStatus" md:"-"`
	InterviewLive    bool     `json:"interviewLive" md:"-"`
}

type NtfyNews struct {
	Id      string   `json:"id"`
	Time    int      `json:"time"`
	Expires int      `json:"expires"`
	Event   string   `json:"event"`
	Topic   string   `json:"topic"`
	Title   string   `json:"title"`
	Message string   `json:"message"`
	Tags    []string `json:"tags"`
	Icon    string   `json:"icon"`
}
