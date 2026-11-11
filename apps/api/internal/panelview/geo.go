package panelview

// countryCentroid returns an English name and a geographic centroid for an ISO code.
// Unknown codes are returned unchanged and ok is false so the map does not invent a point.
func countryCentroid(code string) (name string, lat, lon float64, ok bool) {
	item, found := countries[code]
	if !found {
		if code == "" {
			return "Unknown", 0, 0, false
		}
		return code, 0, 0, false
	}
	return item.name, item.lat, item.lon, true
}

type centroid struct {
	name string
	lat  float64
	lon  float64
}

var countries = map[string]centroid{
	"US": {"United States", 39.8, -98.6},
	"DE": {"Germany", 51.2, 10.4},
	"GB": {"United Kingdom", 54.0, -2.5},
	"NL": {"Netherlands", 52.1, 5.3},
	"FR": {"France", 46.2, 2.2},
	"SG": {"Singapore", 1.35, 103.8},
	"JP": {"Japan", 36.2, 138.3},
	"IN": {"India", 22.0, 79.0},
	"BR": {"Brazil", -10.8, -53.0},
	"CA": {"Canada", 56.1, -106.3},
	"AU": {"Australia", -25.3, 133.8},
	"CN": {"China", 35.9, 104.2},
	"RU": {"Russia", 61.5, 105.3},
	"KR": {"South Korea", 36.5, 127.9},
	"MX": {"Mexico", 23.6, -102.5},
	"FI": {"Finland", 64.0, 26.0},
	"ZA": {"South Africa", -30.6, 22.9},
	"NG": {"Nigeria", 9.1, 8.7},
	"SE": {"Sweden", 62.2, 15.2},
	"NO": {"Norway", 64.5, 12.0},
	"ES": {"Spain", 40.2, -3.7},
	"IT": {"Italy", 42.8, 12.6},
	"PL": {"Poland", 52.1, 19.4},
	"IE": {"Ireland", 53.1, -8.0},
	"CH": {"Switzerland", 46.8, 8.2},
	"AT": {"Austria", 47.6, 14.1},
	"BE": {"Belgium", 50.5, 4.5},
	"DK": {"Denmark", 56.2, 10.0},
	"PT": {"Portugal", 39.4, -8.2},
	"CZ": {"Czechia", 49.8, 15.5},
	"RO": {"Romania", 45.9, 25.0},
	"UA": {"Ukraine", 48.4, 31.2},
	"TR": {"Turkey", 39.0, 35.2},
	"IL": {"Israel", 31.0, 34.9},
	"AE": {"United Arab Emirates", 24.0, 54.0},
	"SA": {"Saudi Arabia", 24.0, 45.0},
	"HK": {"Hong Kong", 22.3, 114.2},
	"TW": {"Taiwan", 23.7, 121.0},
	"ID": {"Indonesia", -2.5, 118.0},
	"TH": {"Thailand", 15.9, 100.9},
	"VN": {"Vietnam", 14.1, 108.3},
	"PH": {"Philippines", 12.9, 121.8},
	"MY": {"Malaysia", 4.2, 102.0},
	"NZ": {"New Zealand", -41.0, 174.0},
	"AR": {"Argentina", -38.4, -63.6},
	"CL": {"Chile", -35.7, -71.5},
	"CO": {"Colombia", 4.6, -74.3},
	"EG": {"Egypt", 26.8, 30.8},
	"KE": {"Kenya", 0.0, 37.9},
	"PK": {"Pakistan", 30.4, 69.3},
	"BD": {"Bangladesh", 23.7, 90.4},
}
