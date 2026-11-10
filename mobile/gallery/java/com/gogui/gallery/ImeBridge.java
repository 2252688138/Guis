package com.gogui.gallery;

import android.app.Activity;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.ViewGroup;
import android.view.inputmethod.BaseInputConnection;
import android.view.inputmethod.InputMethodManager;
import android.widget.EditText;
import android.widget.FrameLayout;

/**
 * NativeActivity 的输入法（IME）桥。
 *
 * <p>为什么需要它：NativeActivity 没有 InputConnection，系统输入法提交的
 * commitText（中文候选上屏就是这一步）根本进不了 AInputQueue，原生输入框看不到任何字符。
 * 这里在窗口顶层挂一个 1x1 的透明 EditText 承接输入法，把「上屏后的整段文本」通过 JNI
 * 回调给 Go 侧，Go 侧再按差异注入组件库的焦点输入框。
 *
 * <p>调用方向：
 *  - Go → Java：{@link #show}(mode, text) / {@link #hide}() / {@link #sync}(text)
 *  - Java → Go：{@link #nativeText}(text) 只在「非组合态」回调，
 *    组合中的拼音串不会回调（避免把拼音当正文插进画布）。
 *
 * <p>线程：所有 View 操作都 post 到 UI 线程（Go 侧调用来自渲染线程，直接 addView 会抛
 * CalledFromWrongThreadException）。
 */
public final class ImeBridge {

    /** show 的 mode：单行（默认）/ 多行 / 密码。 */
    public static final int MODE_TEXT = 0;
    public static final int MODE_MULTILINE = 1;
    public static final int MODE_PASSWORD = 2;

    private final Activity activity;
    private EditText edit;
    /** selfSync=true 表示这次文本变化是「Go 侧同步过来的」，不能回传给 Go（否则回环）。 */
    private boolean selfSync;

    public ImeBridge(Activity activity) {
        this.activity = activity;
    }

    /** 创建隐藏的 EditText（必须在 UI 线程）。 */
    private void ensureEdit() {
        if (edit != null) {
            return;
        }
        edit = new EditText(activity);
        edit.setBackgroundColor(0x00000000);
        edit.setTextColor(0x00000000);
        edit.setHintTextColor(0x00000000);
        edit.setCursorVisible(false);
        edit.setAlpha(0.01f);
        edit.setSingleLine(false);
        FrameLayout.LayoutParams lp = new FrameLayout.LayoutParams(1, 1);
        lp.gravity = Gravity.LEFT | Gravity.BOTTOM;
        edit.setLayoutParams(lp);
        edit.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {
            }

            @Override
            public void onTextChanged(CharSequence s, int start, int before, int count) {
            }

            @Override
            public void afterTextChanged(Editable s) {
                if (selfSync || s == null) {
                    return;
                }
                // 组合态（拼音还没选词）：先不回调，等上屏后整段提交
                int cs = BaseInputConnection.getComposingSpanStart(s);
                int ce = BaseInputConnection.getComposingSpanEnd(s);
                if (cs != -1 && cs != ce) {
                    return;
                }
                nativeText(s.toString());
            }
        });
        ViewGroup root = (ViewGroup) activity.getWindow().getDecorView();
        root.addView(edit);
    }

    /** 弹出输入法，并把当前输入框内容预置进去（让输入法能做联想 / 续写）。 */
    public void show(final int mode, final String text) {
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                ensureEdit();
                selfSync = true;
                edit.setText(text != null ? text : "");
                edit.setSelection(edit.getText().length());
                selfSync = false;
                edit.setInputType(inputType(mode));
                edit.requestFocus();
                InputMethodManager imm =
                        (InputMethodManager) activity.getSystemService(Activity.INPUT_METHOD_SERVICE);
                if (imm != null) {
                    imm.showSoftInput(edit, InputMethodManager.SHOW_IMPLICIT);
                }
            }
        });
    }

    /** 收起输入法。 */
    public void hide() {
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (edit == null) {
                    return;
                }
                InputMethodManager imm =
                        (InputMethodManager) activity.getSystemService(Activity.INPUT_METHOD_SERVICE);
                if (imm != null) {
                    imm.hideSoftInputFromWindow(edit.getWindowToken(), 0);
                }
            }
        });
    }

    /** 把 Go 侧的最新文本同步进来（例如输入框被程序改写、提交后归一化）。 */
    public void sync(final String text) {
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                if (edit == null) {
                    return;
                }
                selfSync = true;
                edit.setText(text != null ? text : "");
                edit.setSelection(edit.getText().length());
                selfSync = false;
            }
        });
    }

    private static int inputType(int mode) {
        if (mode == MODE_PASSWORD) {
            return InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD;
        }
        if (mode == MODE_MULTILINE) {
            return InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_FLAG_MULTI_LINE;
        }
        return InputType.TYPE_CLASS_TEXT;
    }

    /** 文本上屏回调（由 native 侧实现）。 */
    private static native void nativeText(String text);
}
