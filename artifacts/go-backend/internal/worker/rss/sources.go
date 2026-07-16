package rss

// Source defines one RSS feed to fetch from.
type Source struct {
	Name            string
	URL             string
	Region          string // "india" or "global"
	Category        string
	IntervalMinutes int
	Trust           int
}

// FeedSources is the complete list of 170+ RSS sources covering:
//   - India: Startup/VC, Markets/Business, Regulatory, Sector verticals
//   - Global: Startup/VC, Markets/Finance, Technology, Crypto
//
// Sources are listed in trust-priority order within each group.
var FeedSources = []Source{

	// ── INDIA STARTUP & VC ────────────────────────────────────────────────────
	{Name: "Inc42", URL: "https://inc42.com/feed/", Region: "india", Category: "startup", IntervalMinutes: 5, Trust: 12},
	{Name: "Inc42 Features", URL: "https://inc42.com/features/feed/", Region: "india", Category: "startup", IntervalMinutes: 10, Trust: 11},
	{Name: "Inc42 Buzz", URL: "https://inc42.com/buzz/feed/", Region: "india", Category: "startup", IntervalMinutes: 15, Trust: 11},
	{Name: "YourStory", URL: "https://yourstory.com/feed", Region: "india", Category: "startup", IntervalMinutes: 5, Trust: 11},
	{Name: "YourStory Funding", URL: "https://yourstory.com/category/funding/feed", Region: "india", Category: "funding", IntervalMinutes: 5, Trust: 12},
	{Name: "The Ken", URL: "https://the-ken.com/feed/", Region: "india", Category: "startup", IntervalMinutes: 30, Trust: 10},
	{Name: "DealStreetAsia India", URL: "https://www.dealstreetasia.com/tag/india/feed/", Region: "india", Category: "funding", IntervalMinutes: 15, Trust: 10},
	{Name: "Tech in Asia India", URL: "https://www.techinasia.com/tag/india/feed", Region: "india", Category: "startup", IntervalMinutes: 15, Trust: 9},
	{Name: "StartupTalky", URL: "https://startuptalky.com/rss/", Region: "india", Category: "startup", IntervalMinutes: 15, Trust: 8},
	{Name: "IPO Watch", URL: "https://ipowatch.in/feed/", Region: "india", Category: "ipo", IntervalMinutes: 10, Trust: 9},

	// ── INDIA ECONOMIC TIMES (sector verticals) ───────────────────────────────
	{Name: "ET Top Stories", URL: "https://economictimes.indiatimes.com/rssfeedstopstories.cms", Region: "india", Category: "business", IntervalMinutes: 5, Trust: 10},
	{Name: "ET Markets", URL: "https://economictimes.indiatimes.com/markets/rssfeeds/1977021501.cms", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 10},
	{Name: "ET Economy", URL: "https://economictimes.indiatimes.com/news/economy/rssfeeds/1373380680.cms", Region: "india", Category: "policy", IntervalMinutes: 5, Trust: 10},
	{Name: "ET Tech Startups", URL: "https://economictimes.indiatimes.com/tech/startups/rssfeeds/101849316.cms", Region: "india", Category: "startup", IntervalMinutes: 5, Trust: 10},
	{Name: "ET Tech Funding", URL: "https://economictimes.indiatimes.com/tech/funding/rssfeeds/101849320.cms", Region: "india", Category: "funding", IntervalMinutes: 5, Trust: 10},
	{Name: "ET SME", URL: "https://economictimes.indiatimes.com/small-biz/startups/rssfeeds/5575607.cms", Region: "india", Category: "startup", IntervalMinutes: 10, Trust: 9},
	{Name: "ET Politics", URL: "https://economictimes.indiatimes.com/news/politics-and-nation/rssfeeds/1052732854.cms", Region: "india", Category: "policy", IntervalMinutes: 15, Trust: 9},
	{Name: "ET International", URL: "https://economictimes.indiatimes.com/news/international/rssfeeds/2647163.cms", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 9},
	{Name: "ET AI", URL: "https://economictimes.indiatimes.com/tech/artificial-intelligence/rssfeeds/109758014.cms", Region: "india", Category: "technology", IntervalMinutes: 10, Trust: 9},
	{Name: "ET Now", URL: "https://economictimes.indiatimes.com/et-now/rssfeeds/22322026.cms", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 9},
	{Name: "ET Telecom", URL: "https://telecom.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "technology", IntervalMinutes: 15, Trust: 9},
	{Name: "ET Energy", URL: "https://energy.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 9},
	{Name: "ET Realty", URL: "https://realty.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 9},
	{Name: "ET CFO", URL: "https://cfo.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "markets", IntervalMinutes: 15, Trust: 9},
	{Name: "ET Government", URL: "https://government.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "policy", IntervalMinutes: 15, Trust: 9},
	{Name: "ET CIO", URL: "https://cio.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "technology", IntervalMinutes: 15, Trust: 9},
	{Name: "ET Auto", URL: "https://auto.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 8},
	{Name: "ET Retail", URL: "https://retail.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 8},
	{Name: "ET Health", URL: "https://health.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 8},
	{Name: "ET BFSI", URL: "https://bfsi.economictimes.indiatimes.com/rss/topstories", Region: "india", Category: "markets", IntervalMinutes: 15, Trust: 8},

	// ── INDIA LIVEMINT ────────────────────────────────────────────────────────
	{Name: "LiveMint Markets", URL: "https://www.livemint.com/rss/markets", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 10},
	{Name: "LiveMint Companies", URL: "https://www.livemint.com/rss/companies", Region: "india", Category: "business", IntervalMinutes: 5, Trust: 10},
	{Name: "LiveMint Money", URL: "https://www.livemint.com/rss/money", Region: "india", Category: "markets", IntervalMinutes: 10, Trust: 9},
	{Name: "LiveMint Industry", URL: "https://www.livemint.com/rss/industry", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 9},
	{Name: "LiveMint Economy", URL: "https://www.livemint.com/rss/economy", Region: "india", Category: "policy", IntervalMinutes: 10, Trust: 9},
	{Name: "LiveMint Startups", URL: "https://www.livemint.com/rss/technology", Region: "india", Category: "startup", IntervalMinutes: 10, Trust: 10},
	{Name: "LiveMint Opinion", URL: "https://www.livemint.com/rss/opinion", Region: "india", Category: "markets", IntervalMinutes: 20, Trust: 8},

	// ── INDIA BUSINESS STANDARD ───────────────────────────────────────────────
	{Name: "BS Markets", URL: "https://www.business-standard.com/rss/markets-106.rss", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 10},
	{Name: "BS Companies", URL: "https://www.business-standard.com/rss/companies-101.rss", Region: "india", Category: "business", IntervalMinutes: 5, Trust: 10},
	{Name: "BS Economy", URL: "https://www.business-standard.com/rss/economy-policy-102.rss", Region: "india", Category: "policy", IntervalMinutes: 10, Trust: 10},
	{Name: "BS Startups", URL: "https://www.business-standard.com/rss/start-ups-122.rss", Region: "india", Category: "startup", IntervalMinutes: 10, Trust: 9},
	{Name: "BS Finance", URL: "https://www.business-standard.com/rss/finance-101.rss", Region: "india", Category: "markets", IntervalMinutes: 10, Trust: 9},
	{Name: "BS Technology", URL: "https://www.business-standard.com/rss/technology-108.rss", Region: "india", Category: "technology", IntervalMinutes: 10, Trust: 9},

	// ── INDIA HBL ─────────────────────────────────────────────────────────────
	{Name: "HBL Markets", URL: "https://www.thehindubusinessline.com/markets/feeder/default.rss", Region: "india", Category: "markets", IntervalMinutes: 10, Trust: 9},
	{Name: "HBL Companies", URL: "https://www.thehindubusinessline.com/companies/feeder/default.rss", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 9},
	{Name: "HBL Economy", URL: "https://www.thehindubusinessline.com/economy/feeder/default.rss", Region: "india", Category: "policy", IntervalMinutes: 10, Trust: 9},
	{Name: "HBL InfoTech", URL: "https://www.thehindubusinessline.com/info-tech/feeder/default.rss", Region: "india", Category: "technology", IntervalMinutes: 15, Trust: 8},

	// ── INDIA MONEYCONTROL ────────────────────────────────────────────────────
	{Name: "Moneycontrol Business", URL: "https://www.moneycontrol.com/rss/business.xml", Region: "india", Category: "business", IntervalMinutes: 5, Trust: 9},
	{Name: "Moneycontrol Markets", URL: "https://www.moneycontrol.com/rss/marketreports.xml", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 9},
	{Name: "Moneycontrol Economy", URL: "https://www.moneycontrol.com/rss/economy.xml", Region: "india", Category: "policy", IntervalMinutes: 10, Trust: 9},
	{Name: "Moneycontrol Startups", URL: "https://www.moneycontrol.com/rss/startups.xml", Region: "india", Category: "startup", IntervalMinutes: 10, Trust: 9},
	{Name: "Moneycontrol MF", URL: "https://www.moneycontrol.com/rss/mutualfunds.xml", Region: "india", Category: "markets", IntervalMinutes: 15, Trust: 8},
	{Name: "Moneycontrol IT", URL: "https://www.moneycontrol.com/rss/technology.xml", Region: "india", Category: "technology", IntervalMinutes: 15, Trust: 8},
	{Name: "Moneycontrol RE", URL: "https://www.moneycontrol.com/rss/realestate.xml", Region: "india", Category: "business", IntervalMinutes: 15, Trust: 8},
	{Name: "Moneycontrol Brokerage", URL: "https://www.moneycontrol.com/rss/brokerageportal.xml", Region: "india", Category: "markets", IntervalMinutes: 15, Trust: 8},

	// ── INDIA CNBCTV18 ────────────────────────────────────────────────────────
	{Name: "CNBCTV18", URL: "https://www.cnbctv18.com/commonfeeds/v1/cne/rss/business.xml", Region: "india", Category: "business", IntervalMinutes: 5, Trust: 9},
	{Name: "CNBCTV18 Markets", URL: "https://www.cnbctv18.com/commonfeeds/v1/cne/rss/market.xml", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 9},
	{Name: "CNBCTV18 Startups", URL: "https://www.cnbctv18.com/commonfeeds/v1/cne/rss/startup.xml", Region: "india", Category: "startup", IntervalMinutes: 10, Trust: 9},
	{Name: "CNBCTV18 Economy", URL: "https://www.cnbctv18.com/commonfeeds/v1/cne/rss/economy.xml", Region: "india", Category: "policy", IntervalMinutes: 10, Trust: 9},

	// ── INDIA OTHER BUSINESS ──────────────────────────────────────────────────
	{Name: "NDTV Profit", URL: "https://feeds.feedburner.com/ndtvprofit-latest", Region: "india", Category: "markets", IntervalMinutes: 5, Trust: 9},
	{Name: "Business Today", URL: "https://www.businesstoday.in/rss/all", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 9},
	{Name: "The Hindu Business", URL: "https://www.thehindu.com/business/feeder/default.rss", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 9},
	{Name: "NDTV Business", URL: "https://feeds.feedburner.com/ndtvbusiness", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 8},
	{Name: "News18 Business", URL: "https://www.news18.com/rss/business.xml", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 8},
	{Name: "ABP Live Business", URL: "https://news.abplive.com/business/feed", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 8},
	{Name: "TOI Business", URL: "https://timesofindia.indiatimes.com/business/rssfeedstopstories.cms", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 8},
	{Name: "Indian Express Business", URL: "https://indianexpress.com/section/business/feed/", Region: "india", Category: "business", IntervalMinutes: 10, Trust: 8},
	{Name: "Money9", URL: "https://money9.com/feed", Region: "india", Category: "markets", IntervalMinutes: 20, Trust: 7},
	{Name: "DNA Business", URL: "https://www.dnaindia.com/business/feed", Region: "india", Category: "business", IntervalMinutes: 20, Trust: 7},
	{Name: "NDTV Gadgets", URL: "https://feeds.feedburner.com/gadgets360-latest", Region: "india", Category: "technology", IntervalMinutes: 10, Trust: 8},
	{Name: "MediaNama", URL: "https://www.medianama.com/feed/", Region: "india", Category: "technology", IntervalMinutes: 15, Trust: 9},
	{Name: "Zerodha Z-Connect", URL: "https://zerodha.com/z-connect/feed", Region: "india", Category: "markets", IntervalMinutes: 20, Trust: 8},
	{Name: "Finshots", URL: "https://finshots.in/feed/", Region: "india", Category: "markets", IntervalMinutes: 20, Trust: 8},
	{Name: "Value Research", URL: "https://www.valueresearchonline.com/rss/", Region: "india", Category: "markets", IntervalMinutes: 20, Trust: 9},
	{Name: "AMBCrypto", URL: "https://ambcrypto.com/feed/", Region: "india", Category: "crypto", IntervalMinutes: 15, Trust: 7},
	{Name: "CoinGape", URL: "https://coingape.com/feed/", Region: "india", Category: "crypto", IntervalMinutes: 15, Trust: 7},

	// ── INDIA OFFICIAL / REGULATORY ───────────────────────────────────────────
	// PIB's old RSS endpoint 302s to its own error page with no working
	// replacement found — removed rather than left permanently failing.
	{Name: "RBI", URL: "https://www.rbi.org.in/pressreleases_rss.xml", Region: "india", Category: "policy", IntervalMinutes: 15, Trust: 15},
	{Name: "SEBI", URL: "https://www.sebi.gov.in/sebirss.xml", Region: "india", Category: "policy", IntervalMinutes: 20, Trust: 15},

	// ── GLOBAL STARTUP & VC ───────────────────────────────────────────────────
	{Name: "TechCrunch", URL: "https://techcrunch.com/feed/", Region: "global", Category: "startup", IntervalMinutes: 5, Trust: 10},
	{Name: "TechCrunch Startups", URL: "https://techcrunch.com/category/startups/feed/", Region: "global", Category: "startup", IntervalMinutes: 5, Trust: 10},
	{Name: "TechCrunch Funding", URL: "https://techcrunch.com/tag/funding/feed/", Region: "global", Category: "funding", IntervalMinutes: 5, Trust: 10},
	{Name: "TechCrunch Venture", URL: "https://techcrunch.com/category/venture/feed/", Region: "global", Category: "funding", IntervalMinutes: 10, Trust: 10},
	{Name: "Crunchbase News", URL: "https://news.crunchbase.com/feed/", Region: "global", Category: "funding", IntervalMinutes: 15, Trust: 10},
	{Name: "a16z", URL: "https://future.a16z.com/feed", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 10},
	{Name: "YC Blog", URL: "https://blog.ycombinator.com/feed/", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 10},
	{Name: "Sifted", URL: "https://sifted.eu/feed", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 9},
	{Name: "VentureBeat", URL: "http://feeds.feedburner.com/venturebeat/SZYF", Region: "global", Category: "startup", IntervalMinutes: 10, Trust: 8},
	{Name: "CB Insights", URL: "https://www.cbinsights.com/research/feed/", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 9},
	{Name: "Rest of World", URL: "https://restofworld.org/feed/", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 9},
	{Name: "Finsmes", URL: "https://finsmes.com/feed/", Region: "global", Category: "funding", IntervalMinutes: 10, Trust: 8},
	{Name: "Tech.eu", URL: "https://tech.eu/feed/", Region: "global", Category: "funding", IntervalMinutes: 20, Trust: 8},
	{Name: "EU-Startups", URL: "https://www.eu-startups.com/feed/", Region: "global", Category: "startup", IntervalMinutes: 20, Trust: 8},
	{Name: "Inc Magazine", URL: "https://www.inc.com/rss/", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 8},
	{Name: "Entrepreneur", URL: "https://www.entrepreneur.com/latest.rss", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 7},
	{Name: "Fast Company", URL: "https://www.fastcompany.com/latest/rss", Region: "global", Category: "startup", IntervalMinutes: 30, Trust: 8},
	{Name: "Fortune", URL: "https://fortune.com/feed/", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 8},
	{Name: "Harvard Business Review", URL: "https://feeds.hbr.org/harvardbusiness", Region: "global", Category: "business", IntervalMinutes: 30, Trust: 9},
	{Name: "Forbes Business", URL: "https://www.forbes.com/business/feed/", Region: "global", Category: "business", IntervalMinutes: 20, Trust: 8},
	{Name: "Forbes Innovation", URL: "https://www.forbes.com/innovation/feed/", Region: "global", Category: "technology", IntervalMinutes: 20, Trust: 8},
	{Name: "Quartz", URL: "https://qz.com/feed/", Region: "global", Category: "business", IntervalMinutes: 20, Trust: 8},

	// ── GLOBAL MARKETS & FINANCE ──────────────────────────────────────────────
	// Reuters (feeds.reuters.com) and AP (feeds.apnews.com) retired their
	// public RSS domains years ago — DNS no longer resolves for either, for
	// anyone, not just this fetcher. Bloomberg's public RSS is likewise gone
	// (404 on every path tried, including the bare /feed root). No live
	// replacement exists for any of the three; removed rather than left
	// permanently failing.
	{Name: "Fox Business", URL: "https://feeds.foxnews.com/foxbusiness/latest", Region: "global", Category: "business", IntervalMinutes: 10, Trust: 9},
	{Name: "IBTimes", URL: "https://www.ibtimes.com/rss", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 8},
	{Name: "SCMP Business", URL: "https://www.scmp.com/rss/92/feed", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 9},
	{Name: "Insider Monkey", URL: "https://www.insidermonkey.com/blog/feed", Region: "global", Category: "markets", IntervalMinutes: 15, Trust: 7},
	{Name: "Insurance Business Asia", URL: "https://www.insurancebusinessmag.com/asia/rss/", Region: "global", Category: "markets", IntervalMinutes: 20, Trust: 7},
	{Name: "CNBC Markets", URL: "https://www.cnbc.com/id/100003241/device/rss/rss.html", Region: "global", Category: "markets", IntervalMinutes: 5, Trust: 9},
	{Name: "CNBC Finance", URL: "https://www.cnbc.com/id/10000664/device/rss/rss.html", Region: "global", Category: "markets", IntervalMinutes: 5, Trust: 9},
	{Name: "CNBC World", URL: "https://www.cnbc.com/id/100727362/device/rss/rss.html", Region: "global", Category: "business", IntervalMinutes: 10, Trust: 9},
	{Name: "MarketWatch", URL: "https://feeds.marketwatch.com/marketwatch/topstories/", Region: "global", Category: "markets", IntervalMinutes: 10, Trust: 9},
	{Name: "Yahoo Finance", URL: "https://finance.yahoo.com/news/rssindex", Region: "global", Category: "markets", IntervalMinutes: 10, Trust: 8},
	{Name: "WSJ Business", URL: "https://feeds.a.dj.com/rss/WSJcomUSBusiness.xml", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 11},
	{Name: "WSJ Markets", URL: "https://feeds.a.dj.com/rss/RSSMarketsMain.xml", Region: "global", Category: "markets", IntervalMinutes: 15, Trust: 11},
	{Name: "WSJ Tech", URL: "https://feeds.a.dj.com/rss/RSSWSJD.xml", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 11},
	{Name: "FT Business", URL: "https://www.ft.com/rss/home/uk", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 11},
	{Name: "Economist Finance", URL: "https://www.economist.com/finance-and-economics/rss.xml", Region: "global", Category: "markets", IntervalMinutes: 30, Trust: 10},
	{Name: "NYT Business", URL: "https://rss.nytimes.com/services/xml/rss/nyt/Business.xml", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 10},
	{Name: "NYT Economy", URL: "https://rss.nytimes.com/services/xml/rss/nyt/Economy.xml", Region: "global", Category: "policy", IntervalMinutes: 15, Trust: 10},
	{Name: "Guardian Business", URL: "https://www.theguardian.com/uk/business/rss", Region: "global", Category: "business", IntervalMinutes: 15, Trust: 9},
	// Nikkei Asia's RSS path 404s under every pattern tried (asia.nikkei.com
	// paywalled their feed or dropped it entirely) — no live replacement
	// found; removed. Same for Benzinga's old /news/rss/ path — the one
	// working Benzinga feed found (/feed) turns out to be an unrelated
	// auto-generated "crypto price prediction 2025-2030" content stream,
	// not real news, so it was left out rather than substituted in.
	{Name: "Barron's", URL: "https://www.barrons.com/xmlfeed/rss/mktNews.rss", Region: "global", Category: "markets", IntervalMinutes: 15, Trust: 9},
	{Name: "Seeking Alpha", URL: "https://seekingalpha.com/market_currents.xml", Region: "global", Category: "markets", IntervalMinutes: 20, Trust: 8},
	{Name: "Investing.com Markets", URL: "https://www.investing.com/rss/news_1.rss", Region: "global", Category: "markets", IntervalMinutes: 15, Trust: 7},
	{Name: "Investing.com Crypto", URL: "https://www.investing.com/rss/news_301.rss", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 7},
	{Name: "Investing.com Forex", URL: "https://www.investing.com/rss/news_2.rss", Region: "global", Category: "markets", IntervalMinutes: 15, Trust: 7},
	{Name: "Global Finance", URL: "https://gfmag.com/feed/", Region: "global", Category: "markets", IntervalMinutes: 30, Trust: 8},

	// ── GLOBAL TECH ───────────────────────────────────────────────────────────
	{Name: "Wired Business", URL: "https://www.wired.com/feed/category/business/latest/rss", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 8},
	{Name: "The Verge Tech", URL: "https://www.theverge.com/rss/index.xml", Region: "global", Category: "technology", IntervalMinutes: 10, Trust: 8},
	{Name: "MIT Tech Review", URL: "https://www.technologyreview.com/feed/", Region: "global", Category: "technology", IntervalMinutes: 30, Trust: 9},
	{Name: "Hacker News", URL: "https://news.ycombinator.com/rss", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 6},
	{Name: "Techmeme", URL: "https://www.techmeme.com/feed.xml", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 7},
	{Name: "Ars Technica Biz", URL: "https://arstechnica.com/category/biz-it/feed/", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 8},
	{Name: "ZDNet", URL: "https://www.zdnet.com/news/rss.xml", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 7},
	{Name: "The Register", URL: "https://www.theregister.com/headlines.atom", Region: "global", Category: "technology", IntervalMinutes: 15, Trust: 7},

	// ── CRYPTO ────────────────────────────────────────────────────────────────
	{Name: "CoinDesk", URL: "https://www.coindesk.com/arc/outboundfeeds/rss/", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 8},
	{Name: "CoinTelegraph", URL: "https://cointelegraph.com/rss", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 7},
	{Name: "Decrypt", URL: "https://decrypt.co/feed", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 7},
	{Name: "The Block", URL: "https://www.theblock.co/rss.xml", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 8},
	{Name: "Bitcoin Magazine", URL: "https://bitcoinmagazine.com/feed", Region: "global", Category: "crypto", IntervalMinutes: 20, Trust: 7},
	{Name: "Blockworks", URL: "https://blockworks.co/feed/", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 8},
	{Name: "The Defiant", URL: "https://thedefiant.io/feed", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 8},
	{Name: "NewsBTC", URL: "https://www.newsbtc.com/feed/", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 7},
	{Name: "BeInCrypto", URL: "https://beincrypto.com/feed/", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 7},
	{Name: "Watcher Guru", URL: "https://watcher.guru/news/feed", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 6},
	{Name: "U.Today", URL: "https://u.today/rss", Region: "global", Category: "crypto", IntervalMinutes: 15, Trust: 6},
}

// BroadKeywords is the relevance filter — any match means the article is finance-relevant.
var BroadKeywords = []string{
	"fund", "raise", "vc", "capital", "invest", "seed", "series", "valuation",
	"acquired", "acquisition", "merger", "ipo", "stock", "share", "nifty", "sensex",
	"nse", "bse", "market", "policy", "rate", "inflation", "gdp", "tax",
	"profit", "loss", "revenue", "sales", "earnings", "ebitda", "crore", "lakh",
	"billion", "million", "usd", "inr", "₹", "startup", "founder", "ceo",
	"company", "firm", "tech", "software", "ai", "artificial intelligence", "app",
	"platform", "crypto", "bitcoin", "ethereum", "blockchain", "web3",
	"telecom", "auto", "retail", "energy", "power", "realty", "real estate",
	"bank", "insur", "governm", "sebi", "rbi", "gst",
}

// MarketSymbol is one tracked market instrument.
type MarketSymbol struct {
	Symbol   string
	Name     string
	Exchange string
	Region   string
}

// MarketSymbols is the complete list of tracked instruments.
// All symbols are Yahoo Finance ticker format.
var MarketSymbols = []MarketSymbol{

	// ── India Broad Indices ───────────────────────────────────────────────────
	{Symbol: "^NSEI", Name: "Nifty 50", Exchange: "NSE", Region: "india"},
	{Symbol: "^BSESN", Name: "Sensex", Exchange: "BSE", Region: "india"},
	{Symbol: "^NSEBANK", Name: "Bank Nifty", Exchange: "NSE", Region: "india"},
	{Symbol: "^CNXMIDCAP", Name: "Nifty Midcap 100", Exchange: "NSE", Region: "india"},
	{Symbol: "^NSMIDCP50", Name: "Nifty Midcap 50", Exchange: "NSE", Region: "india"},
	{Symbol: "^INDIAVIX", Name: "India VIX", Exchange: "NSE", Region: "india"},

	// ── India Sector Indices ──────────────────────────────────────────────────
	{Symbol: "NIFTY_IT.NS", Name: "Nifty IT", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_PHARMA.NS", Name: "Nifty Pharma", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_AUTO.NS", Name: "Nifty Auto", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_FMCG.NS", Name: "Nifty FMCG", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_METAL.NS", Name: "Nifty Metal", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_REALTY.NS", Name: "Nifty Realty", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_ENERGY.NS", Name: "Nifty Energy", Exchange: "NSE", Region: "india"},
	{Symbol: "NIFTY_FIN_SERVICE.NS", Name: "Nifty Fin Services", Exchange: "NSE", Region: "india"},

	// ── India Blue Chips ──────────────────────────────────────────────────────
	{Symbol: "RELIANCE.NS", Name: "Reliance Industries", Exchange: "NSE", Region: "india"},
	{Symbol: "TCS.NS", Name: "TCS", Exchange: "NSE", Region: "india"},
	{Symbol: "HDFCBANK.NS", Name: "HDFC Bank", Exchange: "NSE", Region: "india"},
	{Symbol: "ICICIBANK.NS", Name: "ICICI Bank", Exchange: "NSE", Region: "india"},
	{Symbol: "INFY.NS", Name: "Infosys", Exchange: "NSE", Region: "india"},
	{Symbol: "SBIN.NS", Name: "SBI", Exchange: "NSE", Region: "india"},
	{Symbol: "BHARTIARTL.NS", Name: "Bharti Airtel", Exchange: "NSE", Region: "india"},
	{Symbol: "WIPRO.NS", Name: "Wipro", Exchange: "NSE", Region: "india"},
	{Symbol: "ITC.NS", Name: "ITC", Exchange: "NSE", Region: "india"},
	{Symbol: "HINDUNILVR.NS", Name: "HUL", Exchange: "NSE", Region: "india"},

	// ── Global Indices ────────────────────────────────────────────────────────
	{Symbol: "^GSPC", Name: "S&P 500", Exchange: "NYSE", Region: "global"},
	{Symbol: "^IXIC", Name: "NASDAQ", Exchange: "NASDAQ", Region: "global"},
	{Symbol: "^DJI", Name: "Dow Jones", Exchange: "NYSE", Region: "global"},
	{Symbol: "^FTSE", Name: "FTSE 100", Exchange: "LSE", Region: "global"},
	{Symbol: "^N225", Name: "Nikkei 225", Exchange: "TSE", Region: "global"},
	{Symbol: "^HSI", Name: "Hang Seng", Exchange: "HKEX", Region: "global"},
	{Symbol: "^GDAXI", Name: "DAX", Exchange: "XETRA", Region: "global"},

	// ── FX (all vs INR) ───────────────────────────────────────────────────────
	{Symbol: "USDINR=X", Name: "USD/INR", Exchange: "FX", Region: "fx"},
	{Symbol: "EURINR=X", Name: "EUR/INR", Exchange: "FX", Region: "fx"},
	{Symbol: "GBPINR=X", Name: "GBP/INR", Exchange: "FX", Region: "fx"},
	{Symbol: "JPYINR=X", Name: "JPY/INR", Exchange: "FX", Region: "fx"},
	{Symbol: "AUDINR=X", Name: "AUD/INR", Exchange: "FX", Region: "fx"},

	// ── Commodities ───────────────────────────────────────────────────────────
	{Symbol: "GC=F", Name: "Gold", Exchange: "COMEX", Region: "global"},
	{Symbol: "SI=F", Name: "Silver", Exchange: "COMEX", Region: "global"},
	{Symbol: "CL=F", Name: "Crude Oil (WTI)", Exchange: "NYMEX", Region: "global"},
	{Symbol: "NG=F", Name: "Natural Gas", Exchange: "NYMEX", Region: "global"},

	// ── Crypto ────────────────────────────────────────────────────────────────
	{Symbol: "BTC-USD", Name: "Bitcoin", Exchange: "CRYPTO", Region: "crypto"},
	{Symbol: "ETH-USD", Name: "Ethereum", Exchange: "CRYPTO", Region: "crypto"},
	{Symbol: "SOL-USD", Name: "Solana", Exchange: "CRYPTO", Region: "crypto"},
	{Symbol: "BNB-USD", Name: "BNB", Exchange: "CRYPTO", Region: "crypto"},
}
