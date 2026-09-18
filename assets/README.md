# 黑白像素 U / 闪电图标

仅保留 `quadlink78.ico`，内含 16×16、32×32、48×48 图像，使用黑/白两色和像素阶梯边缘，无平滑渐变。中心为小闪电，外部为像素化 U。

U 轮廓参考宇树官网 https://www.unitree.com/unitree-favicon.svg（2026-09-19）；商标归宇树所有。闪电和像素化处理用于区分本项目，项目并非宇树官方软件。

独立库使用 `python3 scripts/render_icon.py` 重新生成；集成库脚本位于 `tools/quadlink78/render_icon.py`。仅输出 ICO，不生成 PNG/SVG。
