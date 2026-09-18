# 78Link 像素标识

- `quadlink78.png`：透明背景的横版 U / 闪电 + **78Link**，936×132 px。按像素边界裁切，没有白底、白框或外部留白；用于中英文 README。
- `quadlink78.ico`：16×16、32×32、48×48 px 透明背景 U / 闪电图标，用于应用小图标。闪电相对上一版左移一格像素，在 U 内更居中。

黑色实心像素加透明区域，无抗锯齿灰边；横版文字保留 `78Link` 拼写，参考宇树官网字标的右倾、粗笔画与几何切角，使用手绘像素字形；不依赖外部字体。ICO 内使用传统位图帧。

U 轮廓参考宇树官网 https://www.unitree.com/unitree-favicon.svg（2026-09-19）；商标归宇树所有。闪电、像素化处理和 78Link 文字用于区分本项目，项目并非宇树官方软件。

独立库使用 `python3 scripts/render_icon.py` 重新生成 PNG 与 ICO（仅需 Pillow）。

文字风格参考：https://www.unitree.com/images/0079f8938336436e955ea3a98c4e1e59.svg 。
