package skins

// Size is the original presentation's design size, before desktop scaling.
func Size(id string) (int, int) {
	switch id {
	case "zoom-player":
		return 638, 454
	case "zoom-player-silver":
		return 412, 363
	case "zoom-player-fusion":
		return 618, 427
	case "zoom-player-gtz-hd":
		return 300, 282
	case "zoom-player-brownish":
		return 430, 360
	case "series-9":
		return 346, 344
	case "vlc":
		return 454, 300
	case "quicktime":
		return 360, 370
	case "bsplayer", "powerdvd":
		return 640, 360
	default:
		return 960, 640
	}
}

func SeparateController(id string) bool { return id == "bsplayer" || id == "powerdvd" }
