/**
 * gifenc 类型声明（包内无 .d.ts，官方 API 见 README）。
 */
declare module "gifenc" {
  export type Palette = number[][];

  export interface QuantizeOptions {
    format?: "rgb565" | "rgb444" | "rgba4444";
    clearAlpha?: boolean;
    clearAlphaColor?: number;
    clearAlphaThreshold?: number;
    oneBitAlpha?: boolean | number;
    useSqrt?: boolean;
  }

  export interface WriteFrameOptions {
    palette?: Palette | null;
    transparent?: boolean;
    transparentIndex?: number;
    delay?: number; // ms，内部 /10 → GIF centiseconds
    repeat?: number; // -1=一次, 0=无限, >0=次数
    dispose?: number; // -1=自动
    colorDepth?: number;
    first?: boolean;
  }

  export interface GIFEncoderInstance {
    reset(): void;
    finish(): void;
    bytes(): Uint8Array;
    bytesView(): Uint8Array;
    readonly buffer: ArrayBufferLike;
    writeFrame(
      index: Uint8Array | number[],
      width: number,
      height: number,
      opts?: WriteFrameOptions,
    ): void;
  }

  export function GIFEncoder(opt?: { initialCapacity?: number; auto?: boolean }): GIFEncoderInstance;
  export function quantize(
    rgba: Uint8Array | Uint8ClampedArray,
    maxColors: number,
    opts?: QuantizeOptions,
  ): Palette;
  export function applyPalette(
    rgba: Uint8Array | Uint8ClampedArray,
    palette: Palette,
    format?: "rgb565" | "rgb444" | "rgba4444",
  ): Uint8Array;
  export function prequantize(
    rgba: Uint8Array | Uint8ClampedArray,
    opts?: { roundRGB?: number; roundAlpha?: number; oneBitAlpha?: boolean | number },
  ): void;
  export function snapColorsToPalette(
    palette: Palette,
    knownColors: number[][],
    threshold?: number,
  ): Palette;
}
