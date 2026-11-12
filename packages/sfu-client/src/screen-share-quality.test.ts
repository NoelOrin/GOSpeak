import { describe, expect, it } from "vitest";
import {
	computeScreenShareLayerEncodings,
	DEFAULT_SCREEN_SHARE_QUALITY,
	normalizeScreenShareQuality,
	SCREEN_SHARE_QUALITY_ORDER,
	SCREEN_SHARE_QUALITY_PRESETS,
	SCREEN_SHARE_CAPTURE_RESOLUTION,
	screenShareQualityLabel,
	screenShareQualityPreset,
	screenShareScaleDown,
} from "./screen-share-quality";

describe("screen share quality presets", () => {
	it("exposes every tier in display order, clearest first", () => {
		expect(SCREEN_SHARE_QUALITY_ORDER).toEqual([
			"original",
			"1080p",
			"720p",
			"480p",
		]);
		for (const quality of SCREEN_SHARE_QUALITY_ORDER) {
			expect(SCREEN_SHARE_QUALITY_PRESETS[quality]).toBeTruthy();
		}
	});

	it("caps bitrate monotonically as quality drops", () => {
		const bitrates = SCREEN_SHARE_QUALITY_ORDER.map(
			(q) => SCREEN_SHARE_QUALITY_PRESETS[q].encoding.maxBitrate,
		);
		expect(bitrates).toEqual([...bitrates].sort((a, b) => b - a));
	});

	it("defaults to 1080p", () => {
		expect(DEFAULT_SCREEN_SHARE_QUALITY).toBe("1080p");
		expect(screenShareQualityLabel(DEFAULT_SCREEN_SHARE_QUALITY)).toBe("1080p");
	});

	it("falls back to the default preset for unknown tiers", () => {
		expect(screenShareQualityPreset("4k" as never)).toBe(
			SCREEN_SHARE_QUALITY_PRESETS[DEFAULT_SCREEN_SHARE_QUALITY],
		);
	});
});

describe("normalizeScreenShareQuality", () => {
	it("keeps known tiers", () => {
		expect(normalizeScreenShareQuality("480p")).toBe("480p");
		expect(normalizeScreenShareQuality("original")).toBe("original");
	});

	it("rejects junk persisted values", () => {
		for (const value of [undefined, null, 0, "", "4k", {}, ["720p"]]) {
			expect(normalizeScreenShareQuality(value)).toBe(
				DEFAULT_SCREEN_SHARE_QUALITY,
			);
		}
	});
});

describe("SCREEN_SHARE_CAPTURE_RESOLUTION", () => {
	it("always captures at native size so tiers can be switched in both directions", () => {
		expect(SCREEN_SHARE_CAPTURE_RESOLUTION).toEqual({
			width: 0,
			height: 0,
			frameRate: 30,
		});
	});
});

describe("screenShareScaleDown", () => {
	it("never upscales a capture smaller than the tier", () => {
		expect(screenShareScaleDown(720, 1080)).toBe(1);
		expect(screenShareScaleDown(undefined, 720)).toBe(1);
	});

	it("scales down by the capture-to-tier ratio", () => {
		expect(screenShareScaleDown(2160, 1080)).toBe(2);
		expect(screenShareScaleDown(1440, 720)).toBe(2);
	});

	it("does not scale for the original tier", () => {
		expect(screenShareScaleDown(2160, 0)).toBe(1);
	});
});

describe("computeScreenShareLayerEncodings", () => {
	it("keeps the layer count so rid ordering stays stable", () => {
		for (const layerCount of [1, 2, 3]) {
			expect(
				computeScreenShareLayerEncodings(layerCount, "720p", 1440),
			).toHaveLength(layerCount);
		}
	});

	it("puts the tier bitrate on the top layer and scales lower layers down", () => {
		const layers = computeScreenShareLayerEncodings(3, "1080p", 1080);
		expect(layers.map((l) => l.maxBitrate)).toEqual([
			250_000, 1_000_000, 4_000_000,
		]);
		expect(layers.map((l) => l.scaleResolutionDownBy)).toEqual([4, 2, 1]);
	});

	it("scales the top layer when the capture is larger than the tier", () => {
		const layers = computeScreenShareLayerEncodings(1, "720p", 1440);
		expect(layers[0].scaleResolutionDownBy).toBe(2);
		expect(layers[0].maxBitrate).toBe(2_000_000);
	});

	it("respects the floor bitrate for lowest layers", () => {
		const layers = computeScreenShareLayerEncodings(3, "480p", 1080);
		for (const layer of layers) {
			expect(layer.maxBitrate).toBeGreaterThanOrEqual(150_000);
		}
	});
});
