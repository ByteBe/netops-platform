// 图形验证码：生成一次性 PNG 验证码（5 分钟过期，登录校验后立即作废）
package auth

import (
	"bytes"
	crand "crypto/rand"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"netops/internal/common/response"
)

// 字符集：去除易混淆的 0/O/1/I
var captchaChars = []byte("23456789ABCDEFGHJKLMNPQRSTUVWXYZ")

// 5x7 点阵字体（每字符 N 字节 = N 行，bit0..bit6 自左向右）
var captchaFont = map[byte][]byte{
	'2': {0x0E, 0x11, 0x01, 0x02, 0x04, 0x08, 0x1F},
	'3': {0x0E, 0x11, 0x01, 0x0E, 0x01, 0x11, 0x0E},
	'4': {0x02, 0x06, 0x0A, 0x12, 0x1F, 0x02, 0x02},
	'5': {0x1F, 0x10, 0x1E, 0x01, 0x01, 0x11, 0x0E},
	'6': {0x06, 0x08, 0x10, 0x1E, 0x11, 0x11, 0x0E},
	'7': {0x1F, 0x01, 0x02, 0x04, 0x08, 0x08, 0x08},
	'8': {0x0E, 0x11, 0x11, 0x0E, 0x11, 0x11, 0x0E},
	'9': {0x0E, 0x11, 0x11, 0x0F, 0x01, 0x02, 0x06},
	'A': {0x0E, 0x11, 0x11, 0x1F, 0x11},
	'B': {0x1F, 0x11, 0x1F, 0x11, 0x1F},
	'C': {0x0E, 0x11, 0x10, 0x11, 0x0E},
	'D': {0x1E, 0x11, 0x11, 0x11, 0x1E},
	'E': {0x1F, 0x10, 0x1F, 0x10, 0x1F},
	'F': {0x1F, 0x10, 0x1E, 0x10, 0x10},
	'G': {0x0E, 0x10, 0x17, 0x11, 0x0F},
	'H': {0x11, 0x11, 0x1F, 0x11, 0x11},
	'J': {0x01, 0x01, 0x01, 0x11, 0x0E},
	'K': {0x11, 0x12, 0x14, 0x18, 0x16},
	'L': {0x10, 0x10, 0x10, 0x10, 0x1F},
	'M': {0x11, 0x1B, 0x15, 0x11, 0x11},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11},
	'P': {0x1F, 0x11, 0x1F, 0x10, 0x10},
	'Q': {0x0E, 0x11, 0x15, 0x12, 0x0D},
	'R': {0x1F, 0x11, 0x1F, 0x12, 0x11},
	'S': {0x0F, 0x10, 0x0E, 0x01, 0x1E},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04},
	'U': {0x11, 0x11, 0x11, 0x11, 0x0E},
	'V': {0x11, 0x11, 0x11, 0x0A, 0x04},
	'W': {0x11, 0x11, 0x15, 0x15, 0x0A},
	'X': {0x11, 0x0A, 0x04, 0x0A, 0x11},
	'Y': {0x11, 0x0A, 0x04, 0x04, 0x04},
	'Z': {0x1F, 0x02, 0x04, 0x08, 0x1F},
}

type captchaEntry struct {
	code     string
	expireAt time.Time
}

var captchaStore = struct {
	sync.Mutex
	m map[string]*captchaEntry
}{m: make(map[string]*captchaEntry)}

func init() {
	go func() {
		tk := time.NewTicker(10 * time.Minute)
		for range tk.C {
			captchaStore.Lock()
			now := time.Now()
			for k, e := range captchaStore.m {
				if now.After(e.expireAt) {
					delete(captchaStore.m, k)
				}
			}
			captchaStore.Unlock()
		}
	}()
}

// secureRandIndex 基于 crypto/rand 生成 [0,n) 安全随机数（消除模偏差；
// crypto 读取失败时兜底 math/rand，正常运行不会发生）
func secureRandIndex(n int) int {
	var b [1]byte
	for {
		if _, err := crand.Read(b[:]); err != nil {
			return int(b[0]) % n
		}
		// 拒绝超出最大 256 倍数范围的值，消除模偏差
		if int(b[0]) < 256-(256%n) {
			return int(b[0]) % n
		}
	}
}

// genCaptchaCode 生成 4 位验证码（字符不重复，便于辨认；crypto/rand 安全随机）
func genCaptchaCode() string {
	chars := append([]byte(nil), captchaChars...)
	// Fisher-Yates 安全洗牌
	for i := len(chars) - 1; i > 0; i-- {
		j := secureRandIndex(i + 1)
		chars[i], chars[j] = chars[j], chars[i]
	}
	return string(chars[:4])
}

// drawCaptchaCode 绘制验证码 PNG（标准库，无外部依赖）
func drawCaptchaCode(code string) ([]byte, error) {
	const w, h = 156, 54
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// 随机浅色背景
	bg := color.RGBA{uint8(228 + rand.Intn(22)), uint8(230 + rand.Intn(20)), uint8(235 + rand.Intn(16)), 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	// 干扰线
	for i := 0; i < 5; i++ {
		lc := color.RGBA{uint8(rand.Intn(200)), uint8(rand.Intn(200)), uint8(rand.Intn(200)), 255}
		x1, y1 := rand.Intn(w), rand.Intn(h)
		x2, y2 := rand.Intn(w), rand.Intn(h)
		drawLine(img, x1, y1, x2, y2, lc)
	}
	// 噪点
	for i := 0; i < 200; i++ {
		img.Set(rand.Intn(w), rand.Intn(h), color.RGBA{uint8(rand.Intn(255)), uint8(rand.Intn(255)), uint8(rand.Intn(255)), 255})
	}

	// 绘制字符（3x 放大，随机颜色/偏移/间隔）
	// 字形数据：每字节 = 一行（bit0 为最左列），按行绘制
	x := 18 + rand.Intn(6)
	yBase := 10 + rand.Intn(6)
	for _, ch := range []byte(code) {
		fc := color.RGBA{uint8(20 + rand.Intn(140)), uint8(20 + rand.Intn(140)), uint8(20 + rand.Intn(140)), 255}
		dy := yBase + rand.Intn(7)
		if len(captchaFont[ch]) == 5 {
			dy += 3 // 5 行字符垂直居中到 7 行高度
		}
		drawChar(img, x, dy, ch, fc)
		x += 7*3 + 8 + rand.Intn(5) // 每字符固定 7 列宽
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawChar 绘制单个字符（3x 放大，y 为字符顶部）
// 字形数据：bit0 为最右列，行 0 为最顶行（自左向右、自上而下显示需翻转列序）
func drawChar(img *image.RGBA, x, y int, ch byte, fc color.Color) {
	rows := captchaFont[ch]
	for ri, row := range rows {
		for ci := 0; ci < 7; ci++ {
			if row&(1<<uint(ci)) == 0 {
				continue
			}
			cc := 6 - ci // 列翻转：bit0 → 最右列
			px := x + cc*3
			py := y + ri*3
			for ddx := 0; ddx < 3; ddx++ {
				for ddy := 0; ddy < 3; ddy++ {
					if px+ddx < img.Bounds().Dx() && py+ddy < img.Bounds().Dy() {
						img.Set(px+ddx, py+ddy, fc)
					}
				}
			}
		}
	}
}

// drawLine Bresenham 画线
func drawLine(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	dx, dy := x2-x1, y2-y1
	steps := dx
	if dx < 0 {
		steps = -dx
	}
	if dy < 0 {
		steps = -dy
	}
	if dy > dx && dy > -dx {
		steps = dy
	}
	if steps == 0 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		x := x1 + dx*i/steps
		y := y1 + dy*i/steps
		if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
			img.Set(x, y, c)
		}
	}
}

// captchaHandler GET /auth/captcha 返回验证码图片
func captchaHandler(c *gin.Context) {
	code := genCaptchaCode()
	id := randHex(16)
	pngData, err := drawCaptchaCode(code)
	if err != nil {
		response.Err(c, err)
		return
	}
	captchaStore.Lock()
	// 防刷保护：条目过多时先清理过期项，仍超限则拒绝生成
	now := time.Now()
	for k, e := range captchaStore.m {
		if now.After(e.expireAt) {
			delete(captchaStore.m, k)
		}
	}
	if len(captchaStore.m) >= 20000 {
		captchaStore.Unlock()
		response.Fail(c, http.StatusTooManyRequests, response.CodeBadRequest, "验证码请求过于频繁，请稍后再试")
		return
	}
	captchaStore.m[id] = &captchaEntry{code: code, expireAt: now.Add(5 * time.Minute)}
	captchaStore.Unlock()
	response.OK(c, gin.H{
		"captcha_id":   id,
		"image_base64": base64.StdEncoding.EncodeToString(pngData),
	})
}

// verifyCaptcha 校验验证码（一次性：校验成功即作废）
func verifyCaptcha(id, code string) bool {
	captchaStore.Lock()
	defer captchaStore.Unlock()
	e, ok := captchaStore.m[id]
	if !ok || time.Now().After(e.expireAt) {
		delete(captchaStore.m, id)
		return false
	}
	delete(captchaStore.m, id)
	return strings.EqualFold(e.code, code)
}

// randHex 生成随机十六进制字符串（验证码 ID，crypto/rand 安全随机）
func randHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[secureRandIndex(16)]
	}
	return string(b)
}
