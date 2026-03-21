package com.example

import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [33])
class HomeScreenRobolectricTest {

    @Test
    fun `renders home screen`() {
        val view = HomeScreen()
        assertThat(view).isNotNull()
    }
}
