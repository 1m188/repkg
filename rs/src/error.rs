// 自定义错误类型，与 C# 原版异常对应。

use thiserror::Error;

/// 未知魔数错误，当读取到不认识的格式标识时返回。
/// 与 C# 原版 UnknownMagicException 对应。
#[derive(Error, Debug)]
#[error("未知魔数：{magic}")]
pub struct UnknownMagicError {
    /// 遇到的实际魔数字符串。
    pub magic: String,
}

/// 枚举值无效错误，当读取到不在定义范围内的枚举值时返回。
/// 与 C# 原版 EnumNotValidException 对应。
#[derive(Error, Debug)]
#[error("无效的枚举值：{value}（类型：{type_name}）")]
pub struct EnumNotValidError {
    /// 遇到的实际数值。
    pub value: i32,
    /// 枚举类型名称。
    pub type_name: String,
}

/// 不安全 TEX 数据错误，当数据超出安全限制时返回。
/// 与 C# 原版 UnsafeTexException 对应。
#[derive(Error, Debug)]
#[error("不安全的 TEX 数据：{message}")]
pub struct UnsafeTexError {
    /// 错误描述信息。
    pub message: String,
}

#[cfg(test)]
#[allow(non_snake_case)]
mod tests {
    use super::*;

    #[test]
    fn 测试未知魔数错误显示() {
        let err = UnknownMagicError {
            magic: "TEXB0099".to_string(),
        };
        assert_eq!(err.to_string(), "未知魔数：TEXB0099");
    }

    #[test]
    fn 测试枚举无效错误显示() {
        let err = EnumNotValidError {
            value: 99,
            type_name: "TexFormat".to_string(),
        };
        assert_eq!(err.to_string(), "无效的枚举值：99（类型：TexFormat）");
    }

    #[test]
    fn 测试不安全错误显示() {
        let err = UnsafeTexError {
            message: "mipmap 字节数超限".to_string(),
        };
        assert_eq!(err.to_string(), "不安全的 TEX 数据：mipmap 字节数超限");
    }

    #[test]
    fn 测试错误可被转换为_std_error() {
        fn takes_error(_: &dyn std::error::Error) {}
        let err = UnknownMagicError {
            magic: "test".to_string(),
        };
        takes_error(&err);
    }
}
