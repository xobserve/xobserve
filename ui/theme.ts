import {
    extendTheme,
    withDefaultColorScheme,
    theme as baseTheme,
} from "@chakra-ui/react";
import { customClasses } from "src/theme/custom-classes";
import layerStyles from "src/theme/layer-styles";
import { overrideColors } from "src/theme/override-colors";
import textStyles from "src/theme/text-styles";
import markdownRender from "src/theme/markdown-render";




const customTheme = extendTheme(
    {
        // 自定义基本颜色
        colors: {
            ...overrideColors,
            brand: overrideColors['cyan'] ?? baseTheme.colors['cyan'],
        },
        semanticTokens: {
            colors: {
              // accent semantic tokens
              accent: { default: 'teal.500', _dark: 'teal.300' },
              'accent-emphasis': { default: 'teal.700', _dark: 'teal.200' },
              'accent-static': 'teal.500',
              'accent-muted': { default: 'teal.300', _dark: 'teal.200' },
              'accent-subtle': { default: 'teal.50', _dark: 'teal.800' },
            }
        }
    },
    // 所有的组件，默认使用上面自定义的 `brand` 颜色主题
    withDefaultColorScheme({ colorScheme: "brand" }),
    // 修改默认的 chakra-ui 组件样式
    {
    },
    {
        // 设置默认使用的主题模式
        config: {
            initialColorMode: "dark",
            useSystemColorMode: false,
        },
        // 预定义一些样式，可以在属性中通过 `layerStyle="xxx"` 的方式引入
        layerStyles: layerStyles(),
        textStyles: textStyles(),
        fonts: {
            heading: "Inter, sans-serif",
            body: "Inter, sans-serif",
        },
        styles: {
            global: (props) => {
                return {
                    // 自定义 class
                    ...customClasses(props),
                    p: {
                        wordBreak: "break-word",
                    },
                    ...markdownRender(props),

                }
            }
        }
    }
);

export default customTheme;