import Toybox.Lang;
import Toybox.Test;

// Arrays compare by reference, so check element by element.
function sameIds(a as Array, b as Array) as Boolean {
    if (a.size() != b.size()) { return false; }
    for (var i = 0; i < a.size(); i++) {
        if (a[i] != b[i]) { return false; }
    }
    return true;
}

(:test)
function aqiLabelOneFormPerPlainCategory(logger as Logger) as Boolean {
    Test.assert(sameIds(AqiLabel.forms(0), [Rez.Strings.Cat0]));
    Test.assert(sameIds(AqiLabel.forms(1), [Rez.Strings.Cat1]));
    Test.assert(sameIds(AqiLabel.forms(3), [Rez.Strings.Cat3]));
    Test.assert(sameIds(AqiLabel.forms(4), [Rez.Strings.Cat4]));
    Test.assert(sameIds(AqiLabel.forms(5), [Rez.Strings.Cat5]));
    return true;
}

(:test)
function aqiLabelSensitiveGroupsHasShorterForms(logger as Logger) as Boolean {
    Test.assert(sameIds(AqiLabel.forms(2), [Rez.Strings.Cat2, Rez.Strings.Cat2Mid, Rez.Strings.Cat2Short]));
    Test.assert(!sameIds(AqiLabel.forms(2), [Rez.Strings.Cat2]));
    return true;
}
