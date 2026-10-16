/**
 * .tex-json 元数据生成。
 *
 * 对应 C# RePKG.Application.Texture.TexJsonInfoGenerator。
 * 字段名、取值形态（bool 与字符串混合）与 C# 一致：
 *   bleedtransparentcolors / clampuvs 为 JSON bool；
 *   nomip / nointerpolation / nonpoweroftwo / format 为字符串；
 *   GIF 时追加 spritesheetsequences 数组。
 */
import { TexFlags } from "./enums";
import { texFirstImage, texIsGif, type Tex } from "./tex";

/** 判断非负整数是否为 2 的幂（与 C# NumberIsPowerOfTwo 一致：0 → false） */
function numberIsPowerOfTwo(n: number): boolean {
  if (n === 0) return false;
  while (n !== 1) {
    if (n % 2 !== 0) return false;
    n /= 2;
  }
  return true;
}

type TexJsonField = string | boolean | number | TexJsonField[] | { [k: string]: TexJsonField };

/** 生成与 C# 结构/取值一致的元数据对象 */
export function generateTexJson(tex: Tex): Record<string, TexJsonField> {
  const images = texFirstImage(tex);
  const json: Record<string, TexJsonField> = {
    bleedtransparentcolors: true,
    clampuvs: (tex.header.flags & TexFlags.ClampUVs) === TexFlags.ClampUVs,
    format: texFormatName(tex.header.format),
    nomip: String(images !== null && images.mipmaps.length === 1).toLowerCase(),
    nointerpolation: String((tex.header.flags & TexFlags.NoInterpolation) === TexFlags.NoInterpolation).toLowerCase(),
    nonpoweroftwo: String(
      !numberIsPowerOfTwo(tex.header.imageWidth) || !numberIsPowerOfTwo(tex.header.imageHeight),
    ).toLowerCase(),
  };

  if (texIsGif(tex)) {
    const container = tex.frameInfoContainer;
    if (container === null) {
      throw new Error("TEX is animated but doesn't have frame info container");
    }
    json["spritesheetsequences"] = [
      {
        duration: 1, // C# 注释：not sure what this value is used for
        frames: container.frames.length,
        width: container.gifWidth,
        height: container.gifHeight,
      },
    ];
  }

  return json;
}

/** 生成格式化（2 空格缩进）的 .tex-json 文本，对应 Newtonsoft Formatting.Indented */
export function generateTexJsonText(tex: Tex): string {
  return JSON.stringify(generateTexJson(tex), null, 2);
}

/** TexFormat 枚举名的小写形式（dxt5 / rgba8888 / ...） */
function texFormatName(format: number): string {
  switch (format) {
    case 0:
      return "rgba8888";
    case 4:
      return "dxt5";
    case 6:
      return "dxt3";
    case 7:
      return "dxt1";
    case 8:
      return "rg88";
    case 9:
      return "r8";
    default:
      throw new Error(`TexFormat 无效: ${format}`);
  }
}
