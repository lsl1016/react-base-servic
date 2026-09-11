/**
 * unplugin-icons 虚拟模块类型声明
 *
 * 为 ~icons/* 导入路径提供类型支持
 */

declare module '~icons/*' {
  import type { JSX } from 'solid-js';
  const component: (props: JSX.SvgSVGAttributes<SVGSVGElement>) => JSX.Element;
  export default component;
}