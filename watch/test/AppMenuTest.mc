import Toybox.Lang;
import Toybox.Test;

(:test)
function appMenuVersionLabel(logger as Logger) as Boolean {
    Test.assertEqual(AppMenu.versionLabel("0.0.5", "a19784c"), "0.0.5 (a19784c)");
    return true;
}
