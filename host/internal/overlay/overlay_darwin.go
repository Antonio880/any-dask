//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <math.h>

typedef struct {
    CGFloat x;
    CGFloat y;
    CFTimeInterval start;
    int isAI;
} MacRipple;

@interface AnyDaskLiquidView : NSView
@property (nonatomic, assign) int mode; // 0=hidden, 1=connected, 2=thinking/acting
@property (nonatomic, assign) CGFloat phase;
@property (nonatomic, strong) NSTimer *timer;
@property (nonatomic, strong) NSVisualEffectView *islandView;
@property (nonatomic, strong) NSTextField *islandLabel;
@end

static MacRipple gRipples[12];
static int gRippleCount = 0;
static CGFloat gOriginX = 0.0;
static CGFloat gOriginY = 0.0;

@implementation AnyDaskLiquidView

- (instancetype)initWithFrame:(NSRect)frameRect {
    self = [super initWithFrame:frameRect];
    if (self) {
        self.wantsLayer = YES;
        self.layer.backgroundColor = [[NSColor clearColor] CGColor];

        // Native macOS Frosted Liquid Glass Dynamic Island (real GPU NSVisualEffectView blur!)
        CGFloat pillW = 230;
        CGFloat pillH = 36;
        NSRect pillRect = NSMakeRect((frameRect.size.width - pillW) / 2.0, frameRect.size.height - pillH - 14.0, pillW, pillH);
        self.islandView = [[NSVisualEffectView alloc] initWithFrame:pillRect];
        self.islandView.material = NSVisualEffectMaterialHUDWindow;
        self.islandView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
        self.islandView.state = NSVisualEffectStateActive;
        self.islandView.wantsLayer = YES;
        self.islandView.layer.cornerRadius = pillH / 2.0;
        self.islandView.layer.masksToBounds = YES;
        self.islandView.layer.borderWidth = 1.2;
        self.islandView.layer.borderColor = [[NSColor colorWithWhite:1.0 alpha:0.28] CGColor];

        self.islandLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(16, 8, pillW - 32, 20)];
        self.islandLabel.bezeled = NO;
        self.islandLabel.drawsBackground = NO;
        self.islandLabel.editable = NO;
        self.islandLabel.selectable = NO;
        self.islandLabel.alignment = NSTextAlignmentCenter;
        self.islandLabel.textColor = [NSColor colorWithWhite:1.0 alpha:0.95];
        self.islandLabel.font = [NSFont systemFontOfSize:12.5 weight:NSFontWeightSemibold];
        self.islandLabel.stringValue = @"AnyDask · Conectado";
        [self.islandView addSubview:self.islandLabel];

        [self addSubview:self.islandView];
    }
    return self;
}

- (BOOL)isOpaque { return NO; }

- (void)drawRect:(NSRect)dirtyRect {
    [[NSColor clearColor] setFill];
    NSRectFill(dirtyRect);

    if (self.mode == 0) return;

    CGContextRef ctx = [[NSGraphicsContext currentContext] CGContext];
    CGRect b = self.bounds;
    BOOL aiActive = (self.mode == 2);
    CGFloat borderW = aiActive ? 22.0 : 12.0;
    CGFloat alphaScale = aiActive ? 0.85 : 0.45;

    // 1. Apple Intelligence Liquid Perimeter Glow (multi-layered smooth rounded stroke)
    NSArray<NSColor *> *palette = @[
        [NSColor colorWithRed:0.39 green:0.82 blue:1.00 alpha:alphaScale],
        [NSColor colorWithRed:0.37 green:0.36 blue:0.90 alpha:alphaScale],
        [NSColor colorWithRed:0.75 green:0.35 blue:0.95 alpha:alphaScale],
        [NSColor colorWithRed:1.00 green:0.42 blue:0.72 alpha:alphaScale * 0.85]
    ];

    for (int i = 0; i < 4; i++) {
        CGFloat inset = 2.0 + i * (borderW / 4.0);
        CGFloat radius = 16.0;
        NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:NSInsetRect(b, inset, inset) xRadius:radius yRadius:radius];
        CGFloat pulse = 0.72 + 0.28 * sin(self.phase * 1.4 + i * 1.1);
        NSColor *c = [palette[i] colorWithAlphaComponent:(alphaScale * (1.0 - i * 0.22) * pulse)];
        [c setStroke];
        path.lineWidth = (borderW / 2.2);
        [path stroke];
    }

    // Crisp specular crystal rim
    NSBezierPath *rim = [NSBezierPath bezierPathWithRoundedRect:NSInsetRect(b, 1.5, 1.5) xRadius:16 yRadius:16];
    [[NSColor colorWithWhite:1.0 alpha:(aiActive ? 0.55 : 0.30)] setStroke];
    rim.lineWidth = 1.5;
    [rim stroke];

    // 2. Refractive Liquid Glass Ripples on remote touch & AI clicks
    CFTimeInterval now = CACurrentMediaTime();
    int writeIdx = 0;
    for (int i = 0; i < gRippleCount; i++) {
        MacRipple rp = gRipples[i];
        double prog = (now - rp.start) / 0.65;
        if (prog < 0.0 || prog >= 1.0) continue;
        gRipples[writeIdx++] = rp;

        double ease = 1.0 - pow(1.0 - prog, 3.0);
        double fade = 1.0 - prog;
        CGFloat outerR = 12.0 + ease * 68.0;
        CGFloat innerR = 6.0 + ease * 42.0;

        // Convert global CoreGraphics top-left (rp.x, rp.y) into local Cocoa bottom-left view coords
        CGFloat cx = rp.x - gOriginX;
        CGFloat cy = b.size.height - (rp.y - gOriginY);

        CGContextSaveGState(ctx);
        NSColor *ringColor = rp.isAI
            ? [NSColor colorWithRed:0.75 green:0.35 blue:0.95 alpha:fade * 0.85]
            : [NSColor colorWithRed:0.43 green:0.86 blue:1.00 alpha:fade * 0.85];
        [ringColor setStroke];
        CGContextSetLineWidth(ctx, 3.0 * fade + 1.0);
        CGContextStrokeEllipseInRect(ctx, CGRectMake(cx - outerR, cy - outerR, outerR * 2, outerR * 2));

        [[NSColor colorWithWhite:1.0 alpha:fade * 0.72] setStroke];
        CGContextSetLineWidth(ctx, 1.8);
        CGContextStrokeEllipseInRect(ctx, CGRectMake(cx - innerR, cy - innerR, innerR * 2, innerR * 2));

        [[NSColor colorWithRed:0.55 green:0.85 blue:1.0 alpha:fade * 0.14] setFill];
        CGContextFillEllipseInRect(ctx, CGRectMake(cx - outerR, cy - outerR, outerR * 2, outerR * 2));
        CGContextRestoreGState(ctx);
    }
    gRippleCount = writeIdx;
}

@end

static NSWindow *gOverlayWin = nil;
static AnyDaskLiquidView *gLiquidView = nil;

static void macOverlayInit(void) {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

    NSScreen *screen = [NSScreen mainScreen];
    NSRect frame = screen ? screen.frame : NSMakeRect(0, 0, 1920, 1080);

    gOverlayWin = [[NSWindow alloc] initWithContentRect:frame
                                              styleMask:NSWindowStyleMaskBorderless
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
    [gOverlayWin setLevel:NSScreenSaverWindowLevel];
    [gOverlayWin setOpaque:NO];
    [gOverlayWin setBackgroundColor:[NSColor clearColor]];
    [gOverlayWin setIgnoresMouseEvents:YES];
    [gOverlayWin setHasShadow:NO];
    [gOverlayWin setCollectionBehavior:(NSWindowCollectionBehaviorCanJoinAllSpaces |
                                        NSWindowCollectionBehaviorStationary |
                                        NSWindowCollectionBehaviorIgnoresCycle)];

    gLiquidView = [[AnyDaskLiquidView alloc] initWithFrame:frame];
    [gOverlayWin setContentView:gLiquidView];
    [gOverlayWin orderOut:nil];

    gLiquidView.timer = [NSTimer scheduledTimerWithTimeInterval:(1.0 / 30.0)
                                                        repeats:YES
                                                          block:^(NSTimer *t) {
        if (gLiquidView.mode > 0) {
            gLiquidView.phase += (gLiquidView.mode == 2 ? 0.12 : 0.055);
            [gLiquidView setNeedsDisplay:YES];
        }
    }];
}

static void macOverlayRun(void) {
    @autoreleasepool {
        macOverlayInit();
        [NSApp run];
    }
}

static void macOverlaySetState(int mode, const char *label) {
    NSString *text = label ? [NSString stringWithUTF8String:label] : @"AnyDask";
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!gOverlayWin || !gLiquidView) return;
        gLiquidView.mode = mode;
        if (mode == 0) {
            [gOverlayWin orderOut:nil];
        } else {
            gLiquidView.islandLabel.stringValue = text;
            gLiquidView.islandView.layer.borderColor = (mode == 2)
                ? [[NSColor colorWithRed:0.45 green:0.84 blue:1.0 alpha:0.65] CGColor]
                : [[NSColor colorWithWhite:1.0 alpha:0.28] CGColor];
            [gOverlayWin orderFrontRegardless];
            [gLiquidView setNeedsDisplay:YES];
        }
    });
}

static void macOverlaySetBounds(int displayIndex, int x, int y, int w, int h) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!gOverlayWin || !gLiquidView) return;
        gOriginX = (CGFloat)x;
        gOriginY = (CGFloat)y;

        // CoreGraphics uses top-left origin of primary display (screens[0]); Cocoa uses bottom-left of primary display.
        NSArray<NSScreen *> *screens = [NSScreen screens];
        NSRect targetFrame = NSMakeRect(0, 0, w > 0 ? w : 1920, h > 0 ? h : 1080);
        if (screens.count > 0) {
            CGFloat primaryH = screens[0].frame.size.height;
            CGFloat cocoaY = primaryH - ((CGFloat)y + (CGFloat)h);
            targetFrame = NSMakeRect((CGFloat)x, cocoaY, (CGFloat)w, (CGFloat)h);
        }

        [gOverlayWin setFrame:targetFrame display:YES];
        NSRect localBounds = NSMakeRect(0, 0, targetFrame.size.width, targetFrame.size.height);
        [gLiquidView setFrame:localBounds];

        // Reposition the Liquid Glass Dynamic Island at the top center of the newly selected monitor
        CGFloat pillW = gLiquidView.islandView.frame.size.width;
        CGFloat pillH = gLiquidView.islandView.frame.size.height;
        gLiquidView.islandView.frame = NSMakeRect((localBounds.size.width - pillW) / 2.0,
                                                  localBounds.size.height - pillH - 14.0,
                                                  pillW, pillH);
        gRippleCount = 0;
        [gLiquidView setNeedsDisplay:YES];
    });
}

static void macOverlayRipple(int x, int y, int isAI) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (!gLiquidView) return;
        if (gRippleCount < 12) {
            gRipples[gRippleCount].x = (CGFloat)x;
            gRipples[gRippleCount].y = (CGFloat)y;
            gRipples[gRippleCount].start = CACurrentMediaTime();
            gRipples[gRippleCount].isAI = isAI;
            gRippleCount++;
        }
        [gLiquidView setNeedsDisplay:YES];
    });
}
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

type darwinOverlay struct {
	mu      sync.Mutex
	enabled bool
	state   State
	label   string
}

// RunMainLoop starts the HTTP/WebRTC server in a goroutine and drives AppKit's NSApplication run loop
// on macOS's main thread so native NSVisualEffectView Liquid Glass overlays render at 60fps.
func RunMainLoop(serverFn func()) {
	runtime.LockOSThread()
	go serverFn()
	C.macOverlayRun()
}

func New(enabled bool) Overlay {
	return &darwinOverlay{
		enabled: enabled,
		state:   StateHidden,
	}
}

func (o *darwinOverlay) SetEnabled(enabled bool) {
	o.mu.Lock()
	o.enabled = enabled
	st := o.state
	lbl := o.label
	o.mu.Unlock()
	o.apply(enabled, st, lbl)
}

func (o *darwinOverlay) IsEnabled() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.enabled
}

func (o *darwinOverlay) SetState(state State, label string) {
	o.mu.Lock()
	o.state = state
	o.label = label
	en := o.enabled
	o.mu.Unlock()
	o.apply(en, state, label)
}

func (o *darwinOverlay) apply(enabled bool, state State, label string) {
	mode := 0
	if enabled {
		switch state {
		case StateConnected:
			mode = 1
			if label == "" {
				label = "AnyDask · Conectado"
			}
		case StateAIThinking, StateAIActing:
			mode = 2
			if label == "" {
				label = "AnyDask AI · Agindo"
			}
		}
	}
	cStr := C.CString(label)
	defer C.free(unsafe.Pointer(cStr))
	C.macOverlaySetState(C.int(mode), cStr)
}

func (o *darwinOverlay) SetBounds(displayIndex, x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	C.macOverlaySetBounds(C.int(displayIndex), C.int(x), C.int(y), C.int(w), C.int(h))
}

func (o *darwinOverlay) TriggerRipple(x, y int, isAI bool) {
	o.mu.Lock()
	en := o.enabled && o.state != StateHidden
	o.mu.Unlock()
	if !en {
		return
	}
	aiFlag := 0
	if isAI {
		aiFlag = 1
	}
	C.macOverlayRipple(C.int(x), C.int(y), C.int(aiFlag))
}

func (o *darwinOverlay) Close() {
	o.SetState(StateHidden, "")
}
