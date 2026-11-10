package com.gogui.gallery;

import android.app.NativeActivity;
import android.os.Bundle;

/**
 * 应用 Activity：NativeActivity 的子类，只做一件事——
 * 在 native 库加载完成（super.onCreate 内）后，把输入法桥的实例交给 native 侧。
 *
 * <p>为什么不让 native 自己去加载 ImeBridge：native 线程（Attach 出来的）
 * 没有 Java 侧的 context ClassLoader，FindClass / loadClass 拿不到 dex 里的应用类
 * （实测 ClassLoader 为 null）。由 Java 侧创建对象再通过 JNI 回传，既简单又稳。
 */
public class GalleryActivity extends NativeActivity {

    private ImeBridge ime;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        // super.onCreate 已经加载了 lib<lib_name>.so 并完成 native 初始化，
        // 此时导出符号已可用。
        ime = new ImeBridge(this);
        setBridge(ime);
    }

    /** 把桥对象交给 native（实现在 internal/android/ime.c）。 */
    private native void setBridge(ImeBridge bridge);
}
